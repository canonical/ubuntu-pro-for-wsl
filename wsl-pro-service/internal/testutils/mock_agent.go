package testutils

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// This file deals with mocking the Windows Agent, and introducing errors
// when necessary.

type options struct {
	sendBadPort                 bool
	dropStreamBeforeSendingPort bool
	dropStreamBeforeFirstRecv   bool
}

// AgentOption is used for optional arguments in New.
type AgentOption func(*options)

// WithSendBadPort orders the WslInstance service mock to send port :0,
// which is not a valid port to-presect.
func WithSendBadPort() AgentOption {
	return func(o *options) {
		o.sendBadPort = true
	}
}

// WithDropStreamBeforeReceivingInfo orders the WslInstance service mock
// to drop the connection before receiving the first info.
func WithDropStreamBeforeReceivingInfo() AgentOption {
	return func(o *options) {
		o.dropStreamBeforeFirstRecv = true
	}
}

// WithDropStreamBeforeSendingPort orders the WslInstance service mock
// to drop the connection before sending the port.
func WithDropStreamBeforeSendingPort() AgentOption {
	return func(o *options) {
		o.dropStreamBeforeSendingPort = true
	}
}

// MockWindowsAgent mocks the windows-agent. It starts a GRPC service that will perform
// the port dance and stay connected. It'll write the port file as well.
//
// You can stop the server maunally, otherwise it'll stop during cleanup.
//
//nolint:revive // testing.T should go before context, regardless of what these linters say.
func MockWindowsAgent(t *testing.T, ctx context.Context, addrFile string, args ...AgentOption) *grpc.Server {
	t.Helper()

	var opts options
	for _, f := range args {
		f(&opts)
	}

	server := grpc.NewServer()
	agentapi.RegisterWSLInstanceServer(server, &wslInstanceMockService{
		opts: opts,
	})

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp4", "localhost:0")
	require.NoError(t, err, "Setup: could not listen to agent address")

	go func() {
		log.Infof(ctx, "MockWindowsAgent: Windows-agent mock serving on %q", lis.Addr().String())

		t.Cleanup(server.Stop)

		if err := server.Serve(lis); err != nil {
			log.Infof(ctx, "MockWindowsAgent: Serve returned an error: %v", err)
		}

		if err := os.Remove(addrFile); err != nil {
			log.Infof(ctx, "MockWindowsAgent: Remove address file returned an error: %v", err)
		}
	}()

	err = os.WriteFile(addrFile, []byte(lis.Addr().String()), 0600)
	require.NoError(t, err, "Setup: could not write listening port file")

	return server
}

type wslInstanceMockService struct {
	agentapi.UnimplementedWSLInstanceServer

	opts options
}

func (s *wslInstanceMockService) Connected(stream agentapi.WSLInstance_ConnectedServer) error {
	ctx := context.Background()

	log.Infof(ctx, "wslInstanceMockService: Received incoming connection")

	if s.opts.dropStreamBeforeFirstRecv {
		log.Infof(ctx, "wslInstanceMockService: dropping stream before first Recv as instructed")
		return nil
	}

	info, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("new connection: did not receive info from WSL distro: %v", err)
	}

	distro := info.GetWslName()
	log.Infof(ctx, "wslInstanceMockService: Connection with %q: received info: %+v", distro, info)

	if s.opts.dropStreamBeforeSendingPort {
		log.Infof(ctx, "wslInstanceMockService: Connection with %q: dropping stream before sending port as instructed", distro)
		return nil
	}

	// Get a port and send it
	lis, err := net.Listen("tcp4", "localhost:")
	if err != nil {
		return fmt.Errorf("could not reserve a port for %q: %v", distro, err)
	}

	var port uint32
	// localhost:0 is a bad address to send, as 0 is not a real port, but rather instructs
	// net.Listen to autoselect a new port; hence defeating the point of pre-autoselection.
	if s.opts.sendBadPort {
		log.Infof(ctx, "wslInstanceMockService: Connection with %q: Sending bad port %d", distro, port)
	} else {
		port, err = portFromAddress(lis.Addr().String())
		if err != nil {
			return fmt.Errorf("could not parse address for %q: %v", distro, err)
		}

		if err := lis.Close(); err != nil {
			return fmt.Errorf("could not close port reserved for %q: %v", distro, err)
		}

		log.Infof(ctx, "wslInstanceMockService: Connection with %q: Reserved port %d", distro, port)
	}

	if err := stream.Send(&agentapi.Port{Port: port}); err != nil {
		return fmt.Errorf("could not send port: %v", err)
	}

	// Connect back
	_, err = net.Dial("tcp4", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("could not dial %q: %v", distro, err)
	}

	log.Infof(ctx, "wslInstanceMockService: Connection with %q: connected back via reserved port", distro)

	// Stay connected
	for {
		_, err = stream.Recv()
		if err != nil {
			log.Infof(ctx, "wslInstanceMockService: Connection with %q ended: %v", distro, err)
			break
		}
		log.Infof(ctx, "wslInstanceMockService: Connection with %q: received info: %+v", distro, info)
	}

	return nil
}

func portFromAddress(addr string) (uint32, error) {
	s := strings.Split(addr, ":")
	p, err := strconv.ParseUint(s[len(s)-1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not parse address %q", addr)
	}
	return uint32(p), nil
}
