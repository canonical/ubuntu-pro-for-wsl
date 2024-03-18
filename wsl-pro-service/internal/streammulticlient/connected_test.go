package streammulticlient_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/streammulticlient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestLifecycle(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// One of these four must be true
		dontServe     bool
		stopWithClose bool
		stopServer    bool

		wantErr bool
	}{
		"Success stopping with Close":  {stopWithClose: true},
		"Success stopping server-side": {stopServer: true},

		"Error dialing when there is no server": {dontServe: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var lc net.ListenConfig
			lis, err := lc.Listen(ctx, "tcp", "localhost:0")
			require.NoError(t, err, "Setup: could not listen")
			defer lis.Close()

			s := grpc.NewServer()
			go func() {
				err := s.Serve(lis)
				if err != nil {
					log.Warningf(ctx, "Serve error: %v", err)
				}
			}()
			defer s.Stop()

			if tc.dontServe {
				// We serve and stop, so that the port is reserved but unused
				s.Stop()
				lis.Close()
			}

			conn, err := grpc.DialContext(ctx, lis.Addr().String(),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err, "Dial should return no error")
			defer conn.Close()

			client, err := streammulticlient.Connect(ctx, conn)
			if tc.wantErr {
				require.Error(t, err, "Connect should have returned an error")
				return
			}
			require.NoError(t, err, "Connect should return no error")

			select {
			case <-client.Done(ctx):
				require.Fail(t, "Done should not have returned yet")
			case <-time.After(5 * time.Second):
			}

			if tc.stopServer {
				s.Stop()
			} else if tc.stopWithClose {
				client.Close()
			}

			select {
			case <-client.Done(ctx):
				return
			case <-time.After(20 * time.Second):
				require.Fail(t, "Done should have returned after stopping the connection")
			}
		})
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", "localhost:0")
	require.NoError(t, err, "Setup: could not listen")
	defer lis.Close()

	service := &agentAPIServer{}

	s := grpc.NewServer()
	agentapi.RegisterWSLInstanceServer(s, service)
	go func() {
		err = s.Serve(lis)
		if err != nil {
			log.Warningf(ctx, "Serve error: %v", err)
		}
	}()
	defer s.Stop()

	conn, err := grpc.DialContext(ctx, lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Setup: Dial should have succeeded")
	defer conn.Close()

	client, err := streammulticlient.Connect(ctx, conn)
	require.NoError(t, err, "Connect should not return an error")

	// Connection is immediate but updating the counts is not: hence the waits
	require.Eventually(t, func() bool { return service.connected.callCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "Should have connected to the Connected stream")

	require.Eventually(t, func() bool { return service.proattachment.callCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "Should have connected to the Pro attachment stream")

	require.Eventually(t, func() bool { return service.landscapeConfig.callCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "Should have connected to the Landscape configuration stream")

	// Test sending messages Server->Client
	err = service.SendProAttachmentCmd("token123")
	require.NoError(t, err, "Sending commands should not fail")

	proMsg, err := client.ProAttachStream().Recv()
	require.NoError(t, err, "RecvProAttachCmd should not return error")
	require.Equal(t, "token123", proMsg.GetToken(), "Mismatch between sent and received Pro token")

	err = service.SendLandscapeConfig("[client]\nhello=world", "uid1234")
	require.NoError(t, err, "Sending commands should not fail")

	lpeMsg, err := client.LandscapeConfigStream().Recv()
	require.NoError(t, err, "RecvLandscapeConfigCmd should not return error")
	require.Equal(t, "[client]\nhello=world", lpeMsg.GetConfig(), "Mismatch between sent and received Landscape config")
	require.Equal(t, "uid1234", lpeMsg.GetHostagentUid(), "Mismatch between sent and received Landscape hostagent UID")

	// Test sending messages Client->Server
	err = client.SendInfo(&agentapi.DistroInfo{})
	require.NoError(t, err, "SendInfo should not return error")
	require.Eventually(t, func() bool { return service.connected.recvCount.Load() >= 1 }, // We already received a message during the handshake
		5*time.Second, 100*time.Millisecond, "The server should have received a distro info message")

	err = client.ProAttachStream().Send(&agentapi.Result{})
	require.NoError(t, err, "SendProAttachCmdResult should not return error")
	require.Eventually(t, func() bool { return service.proattachment.recvCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "The server should have received a result message via the Pro attachment stream")

	err = client.LandscapeConfigStream().Send(&agentapi.Result{})
	require.NoError(t, err, "SendLandscapeConfigCmdResult should not return error")
	require.Eventually(t, func() bool { return service.landscapeConfig.recvCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "The server should have received a result message via the Landscape stream")

	// Disconnect to exercise error cases
	client.Close()
	conn.Close()

	_, err = streammulticlient.Connect(ctx, conn)
	require.Error(t, err, "Connect should return an error when using a closed connection")

	// Test sending messages after disconnecting
	err = client.SendInfo(&agentapi.DistroInfo{})
	require.Error(t, err, "SendInfo should return an error after disconnecting")

	err = client.ProAttachStream().Send(&agentapi.Result{})
	require.Error(t, err, "SendProAttachCmdResult should return an error after disconnecting")

	err = client.LandscapeConfigStream().Send(&agentapi.Result{})
	require.Error(t, err, "SendLandscapeConfigCmdResult should return an error after disconnecting")

	// Test receiving messages after disconnecting
	_, err = client.ProAttachStream().Recv()
	require.Error(t, err, "RecvProAttachCmd should return an error after disconnecting")

	_, err = client.LandscapeConfigStream().Recv()
	require.Error(t, err, "RecvLandscapeConfigCmd should return an error after disconnecting")
}

type agentAPIServer struct {
	agentapi.UnimplementedWSLInstanceServer

	connected       stream
	proattachment   stream
	landscapeConfig stream
}

type stream struct {
	callCount atomic.Uint32
	recvCount atomic.Uint32
	stream    atomic.Value
}

func (s *agentAPIServer) Connected(stream agentapi.WSLInstance_ConnectedServer) error {
	s.connected.callCount.Add(1)
	s.connected.stream.Store(stream)

	for {
		_, err := stream.Recv()
		if err != nil {
			return nil
		}

		s.connected.recvCount.Add(1)
	}
}

func (s *agentAPIServer) ProAttachmentCommands(stream agentapi.WSLInstance_ProAttachmentCommandsServer) error {
	s.proattachment.callCount.Add(1)
	s.proattachment.stream.Store(stream)

	for {
		_, err := stream.Recv()
		if err != nil {
			return nil
		}

		s.proattachment.recvCount.Add(1)
	}
}

func (s *agentAPIServer) LandscapeConfigCommands(stream agentapi.WSLInstance_LandscapeConfigCommandsServer) error {
	s.landscapeConfig.callCount.Add(1)
	s.landscapeConfig.stream.Store(stream)

	for {
		_, err := stream.Recv()
		if err != nil {
			return nil
		}

		s.landscapeConfig.recvCount.Add(1)
	}
}

func (s *agentAPIServer) SendProAttachmentCmd(token string) error {
	stream := s.proattachment.stream.Load()
	if stream == nil {
		return errors.New("stream not connected")
	}

	//nolint:forcetypeassert // This value is always this type (or nil, which we checked already)
	return stream.(agentapi.WSLInstance_ProAttachmentCommandsServer).Send(&agentapi.ProAttachCmd{Token: token})
}

func (s *agentAPIServer) SendLandscapeConfig(config, hostagentUID string) error {
	stream := s.landscapeConfig.stream.Load()
	if stream == nil {
		return errors.New("stream not connected")
	}

	//nolint:forcetypeassert // This value is always this type (or nil, which we checked already)
	return stream.(agentapi.WSLInstance_LandscapeConfigCommandsServer).Send(&agentapi.LandscapeConfigCmd{
		Config:       config,
		HostagentUid: hostagentUID,
	})
}
