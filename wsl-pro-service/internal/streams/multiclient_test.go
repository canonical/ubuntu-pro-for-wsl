package streams_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/streams"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestConnect(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		dontServe bool

		wantErr bool
	}{
		"Success": {},

		"Error dialing an address that is not serving": {dontServe: true, wantErr: true},
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

			service := &agentAPIServer{}

			if !tc.dontServe {
				s := grpc.NewServer()
				agentapi.RegisterWSLInstanceServer(s, service)
				go func() {
					err = s.Serve(lis)
					if err != nil {
						log.Warningf(ctx, "Serve error: %v", err)
					}
				}()
				defer s.Stop()
			}

			conn, err := grpc.NewClient(lis.Addr().String(),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err, "Setup: Creating a client should have succeeded")
			defer conn.Close()

			client, err := streams.Connect(ctx, conn)
			if tc.wantErr {
				require.Error(t, err, "Connect should return an error")
				return
			}
			require.NoError(t, err, "Connect should not return an error")

			// Connection is immediate but updating the counts is not: hence the waits
			require.Eventually(t, func() bool { return service.connected.callCount.Load() >= 1 },
				5*time.Second, 100*time.Millisecond, "Should have connected to the Connected stream")

			require.Eventually(t, func() bool { return service.proattachment.callCount.Load() >= 1 },
				5*time.Second, 100*time.Millisecond, "Should have connected to the Pro attachment stream")

			require.Eventually(t, func() bool { return service.landscapeConfig.callCount.Load() >= 1 },
				5*time.Second, 100*time.Millisecond, "Should have connected to the Landscape configuration stream")

			require.NotNil(t, client.ProAttachStream(), "ProAttachStream should not return nil")
			require.NotNil(t, client.LandscapeConfigStream(), "LandscapeConfigStream should not return nil")
		})
	}
}

func TestSendAndRecv(t *testing.T) {
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

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Setup: Creating a client should have succeeded")
	defer conn.Close()

	client, err := streams.Connect(ctx, conn)
	require.NoError(t, err, "Setup: Connect should not return an error")

	require.Eventually(t, func() bool {
		connReady := service.connected.callCount.Load() > 0
		proReady := service.proattachment.callCount.Load() > 0
		lpeReady := service.landscapeConfig.callCount.Load() > 0
		return connReady && proReady && lpeReady
	}, 10*time.Second, 100*time.Millisecond, "Setup: streams never connected")

	// Test sending messages Server->Client
	err = service.SendProAttachmentCmd("token123")
	require.NoError(t, err, "Sending commands should not fail")

	proMsg, err := client.ProAttachStream().Recv()
	require.NoError(t, err, "ProAttachStream.Recv should not return error")
	require.Equal(t, "token123", proMsg.GetToken(), "Mismatch between sent and received Pro token")

	err = service.SendLandscapeConfig("[client]\nhello=world", "uid1234")
	require.NoError(t, err, "Sending commands should not fail")

	lpeMsg, err := client.LandscapeConfigStream().Recv()
	require.NoError(t, err, "LandscapeConfigStream.Recv should not return error")
	require.Equal(t, "[client]\nhello=world", lpeMsg.GetConfig(), "Mismatch between sent and received Landscape config")
	require.Equal(t, "uid1234", lpeMsg.GetHostagentUid(), "Mismatch between sent and received Landscape hostagent UID")

	// Test sending messages Client->Server
	err = client.SendInfo(&agentapi.DistroInfo{})
	require.NoError(t, err, "SendInfo should not return error")
	require.Eventually(t, func() bool { return service.connected.recvCount.Load() >= 1 }, // We already received a message during the handshake
		5*time.Second, 100*time.Millisecond, "The server should have received a distro info message")

	err = client.ProAttachStream().SendResult(nil)
	require.NoError(t, err, "ProAttachStream.SendResult should not return error")
	require.Eventually(t, func() bool { return service.proattachment.recvCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "The server should have received a result message via the Pro attachment stream")

	err = client.LandscapeConfigStream().SendResult(nil)
	require.NoError(t, err, "LandscapeConfigStream.SendResult should not return error")
	require.Eventually(t, func() bool { return service.landscapeConfig.recvCount.Load() >= 1 },
		5*time.Second, 100*time.Millisecond, "The server should have received a result message via the Landscape stream")

	// Disconnect to exercise error cases
	conn.Close()

	_, err = streams.Connect(ctx, conn)
	require.Error(t, err, "Connect should return an error when using a closed connection")

	// Test sending messages after disconnecting
	err = client.SendInfo(&agentapi.DistroInfo{})
	require.Error(t, err, "SendInfo should return an error after disconnecting")

	err = client.ProAttachStream().SendResult(nil)
	require.Error(t, err, "ProAttachStream.SendResult should return an error after disconnecting")

	err = client.LandscapeConfigStream().SendResult(nil)
	require.Error(t, err, "LandscapeConfigStream.SendResult should return an error after disconnecting")

	// Test receiving messages after disconnecting
	_, err = client.ProAttachStream().Recv()
	require.Error(t, err, "ProAttachStream.Recv should return an error after disconnecting")

	_, err = client.LandscapeConfigStream().Recv()
	require.Error(t, err, "SendResult.Recv should return an error after disconnecting")
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
