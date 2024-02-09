package testutils

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/logstreamer"
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
// You can stop the server manually, otherwise it'll stop during cleanup.
//
//nolint:revive // testing.T should go before context, regardless of what these linters say.
func MockWindowsAgent(t *testing.T, ctx context.Context, addrFile string, args ...AgentOption) (*grpc.Server, *MockAgentData) {
	t.Helper()

	var opts options
	for _, f := range args {
		f(&opts)
	}

	server := grpc.NewServer()
	service := &wslInstanceMockService{
		opts: opts,
	}

	agentapi.RegisterWSLInstanceServer(server, service)

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

	return server, &service.data
}

type wslInstanceMockService struct {
	agentapi.UnimplementedWSLInstanceServer

	opts options
	data MockAgentData
}

// MockAgentData contains some stats about the agent and the connections it made.
type MockAgentData struct {
	// ConnectionCount is the number of times the WSL Pro Service has connected to the stream
	ConnectionCount atomic.Int32

	// ConnectionCount is the number of times the Agent has connected to the WSLInstance service
	BackConnectionCount atomic.Int32

	// RecvCount is the number of completed Recv performed by the Agent
	RecvCount atomic.Int32

	// ReservedPort is the latest port reserved for the WSLProService
	ReservedPort atomic.Uint32
}

func (s *wslInstanceMockService) Connected(stream agentapi.WSLInstance_ConnectedServer) (err error) {
	ctx := context.Background()

	defer func(err *error) {
		if *err != nil {
			log.Warningf(ctx, "wslInstanceMockService: dropped connection: %v", *err)
			return
		}
		log.Info(ctx, "wslInstanceMockService: dropped connection")
	}(&err)

	s.data.ConnectionCount.Add(1)

	log.Infof(ctx, "wslInstanceMockService: Received incoming connection")

	if s.opts.dropStreamBeforeFirstRecv {
		log.Infof(ctx, "wslInstanceMockService: mock error: dropping stream before first Recv")
		return nil
	}

	info, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("new connection: did not receive info from WSL distro: %v", err)
	}
	s.data.RecvCount.Add(1)

	distro := info.GetWslName()
	log.Infof(ctx, "wslInstanceMockService: Connection with %q: received info: %+v", distro, info)

	if s.opts.dropStreamBeforeSendingPort {
		log.Infof(ctx, "connection with %q: mock error: dropping stream before sending port", distro)
		return nil
	}

	// Get a port and send it
	lis, err := net.Listen("tcp4", "localhost:")
	if err != nil {
		return fmt.Errorf("could not reserve a port for %q: %v", distro, err)
	}

	var port int
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

	s.data.ReservedPort.Store(uint32(port))
	if err := stream.Send(&agentapi.Port{Port: uint32(port)}); err != nil {
		return fmt.Errorf("could not send port: %v", err)
	}

	// Connect back
	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Second)
		_, err = net.Dial("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			err = fmt.Errorf("wslInstanceMockService: could not dial %q: %v", distro, err)
			continue
		}
		break
	}

	if err != nil {
		return err
	}

	s.data.BackConnectionCount.Add(1)

	log.Infof(ctx, "wslInstanceMockService: Connection with %q: connected back via reserved port", distro)

	// Stay connected
	for {
		_, err = stream.Recv()
		if err != nil {
			log.Infof(ctx, "wslInstanceMockService: Connection with %q ended: %v", distro, err)
			break
		}
		log.Infof(ctx, "wslInstanceMockService: Connection with %q: received info: %+v", distro, info)
		s.data.RecvCount.Add(1)
	}

	return nil
}

func portFromAddress(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("could not parse address %q", addr)
	}
	return net.LookupPort("tcp4", fmt.Sprint(p))
}
