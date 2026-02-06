package streams_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/streams"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sys, _ := testutils.MockSystem(t)

	agent := testutils.NewMockWindowsAgent(t, ctx, t.TempDir())
	defer agent.Stop()

	conn, err := grpc.NewClient(agent.Listener.Addr().String(),
		grpc.WithTransportCredentials(agent.ClientCredentials))
	require.NoError(t, err, "Setup: could not create a client to the mock windows agent")
	defer conn.Close()

	server := streams.NewServer(ctx, sys, conn)

	service := &mockService{}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(service)
		close(errCh)
	}()

	// Test handshake
	require.Eventually(t, agent.Service.AllConnected, 20*time.Second, 500*time.Millisecond, "Setup: Agent service never became ready")

	// Test receiving a pro token and returning success
	err = agent.Service.ProAttachment.Send(&agentapi.ProAttachCmd{Token: "token345"})
	require.NoError(t, err, "Send should return no error")

	require.Eventually(t, func() bool {
		return len(agent.Service.ProAttachment.History()) > 1
	}, 20*time.Second, 100*time.Millisecond, "Server did not send a response to the Pro attach command")
	require.Empty(t, agent.Service.ProAttachment.History()[1].GetResult(), "ProAttachment should return a successful result")

	// Test receiving a pro token and returning error
	err = agent.Service.ProAttachment.Send(&agentapi.ProAttachCmd{Token: "HARDCODED_FAILURE"})
	require.NoError(t, err, "Send should return no error")

	require.Eventually(t, func() bool {
		return len(agent.Service.ProAttachment.History()) > 2
	}, 20*time.Second, 100*time.Millisecond, "Server did not send a response to the Pro attach command")
	require.NotEmpty(t, agent.Service.ProAttachment.History()[2].GetResult(), "ProAttachment should return an error result")

	// Test receiving a Landscape config and returning success
	err = agent.Service.LandscapeConfig.Send(&agentapi.LandscapeConfigCmd{Config: "hello=world"})
	require.NoError(t, err, "Send should return no error")

	require.Eventually(t, func() bool {
		return len(agent.Service.LandscapeConfig.History()) > 1
	}, 20*time.Second, 100*time.Millisecond, "Server did not send a response to the Pro attach command")
	require.Empty(t, agent.Service.LandscapeConfig.History()[1].GetResult(), "LandscapeConfig should return a successful result")

	// Test receiving a Landscape config and returning error
	err = agent.Service.LandscapeConfig.Send(&agentapi.LandscapeConfigCmd{Config: "HARDCODED_FAILURE"})
	require.NoError(t, err, "Send should return no error")

	require.Eventually(t, func() bool {
		return len(agent.Service.LandscapeConfig.History()) > 2
	}, 20*time.Second, 100*time.Millisecond, "Server did not send a response to the Pro attach command")
	require.NotEmpty(t, agent.Service.LandscapeConfig.History()[2].GetResult(), "LandscapeConfig should return an error result")

	server.GracefulStop()
	select {
	case err := <-errCh:
		require.NoError(t, err, "Serve should not return an error when gracefully stopped")
	case <-time.After(10 * time.Second):
		require.Fail(t, "GracefulStop should interrupt Serve")
	}
}

func TestStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sys, _ := testutils.MockSystem(t)

	agent := testutils.NewMockWindowsAgent(t, ctx, t.TempDir())
	defer agent.Stop()

	conn, err := grpc.NewClient(agent.Listener.Addr().String(),
		grpc.WithTransportCredentials(agent.ClientCredentials))
	require.NoError(t, err, "Setup: could not create a client to the mock windows agent")
	defer conn.Close()

	server := streams.NewServer(ctx, sys, conn)

	service := &mockService{}
	errCh := make(chan error)
	go func() {
		errCh <- server.Serve(service)
		close(errCh)
	}()

	require.Eventually(t, agent.Service.AllConnected, 20*time.Second, 500*time.Millisecond, "Setup: Agent service never became ready")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	service.setBlocking(ctx)

	err = agent.Service.ProAttachment.Send(&agentapi.ProAttachCmd{})
	require.NoError(t, err, "mock agent could not send a pro-attach command")

	err = agent.Service.LandscapeConfig.Send(&agentapi.LandscapeConfigCmd{})
	require.NoError(t, err, "mock agent could not send a landscape-config command")

	// Wait for unary calls to be made
	time.Sleep(10 * time.Second)

	server.Stop()
	select {
	case err := <-errCh:
		require.Error(t, err, "Stop should have interrupted the unary calls")
	case <-time.After(10 * time.Second):
		require.Fail(t, "Stop should interrupt Serve")
	}
}

type mockService struct {
	blockingCalls bool
	mu            sync.RWMutex

	ctx context.Context
}

func (s *mockService) setBlocking(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.blockingCalls = true
	s.ctx = ctx
}

func (s *mockService) ApplyProToken(ctx context.Context, msg *agentapi.ProAttachCmd) error {
	if msg.GetToken() == "HARDCODED_FAILURE" {
		return errors.New("mock error")
	}

	// Mock a slow task that can be cancelled
	// Using a mutex because those calls can race with s.setBlocking.
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.blockingCalls {
		select {
		case <-ctx.Done():
			// Mock task interrupted
			return ctx.Err()
		case <-s.ctx.Done():
			// Mock task completed successfully
		}
	}

	return nil
}

func (s *mockService) ApplyLandscapeConfig(ctx context.Context, msg *agentapi.LandscapeConfigCmd) error {
	if msg.GetConfig() == "HARDCODED_FAILURE" {
		return errors.New("mock error")
	}

	// Mock a slow task that can be cancelled
	// Using a mutex because those calls can race with s.setBlocking.
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.blockingCalls {
		select {
		case <-ctx.Done():
			// Mock task interrupted
			return ctx.Err()
		case <-s.ctx.Done():
			// Mock task completed successfully
		}
	}

	return nil
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T) { testutils.WslInfoMock(t) }
