package wslinstanceservice_test

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

func TestApplyProToken(t *testing.T) {
	t.Parallel()

	type detachResult int
	const (
		detachOK detachResult = iota
		detachAlreadyDetached
		detachErr
	)

	testCases := map[string]struct {
		token             string
		proStatusErr      bool
		getSystemErr      bool
		proDetachErr      detachResult
		attachErr         bool
		ctrlStreamSendErr bool

		wantErr bool
	}{
		"success attaching attached machine":     {token: "123"},
		"success attaching non-attached machine": {token: "123", proDetachErr: detachAlreadyDetached},
		"success detaching attached machine":     {},
		"success detaching non-attached machine": {proDetachErr: detachAlreadyDetached},

		// Attach/detach errors
		"Error calling pro attach": {token: "123", attachErr: true, wantErr: true},
		"Error detaching pro":      {proDetachErr: detachErr, wantErr: true},

		// System info
		"Error calling pro status":         {proStatusErr: true, wantErr: true},
		"Error getting system info":        {getSystemErr: true, wantErr: true},
		"Error cannot send info to stream": {ctrlStreamSendErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			wantSysInfo := &agentapi.DistroInfo{
				WslName:     "TEST_DISTRO",
				Id:          "ubuntu",
				VersionId:   "22.04",
				PrettyName:  "Ubuntu 22.04.1 LTS",
				ProAttached: true,
				Hostname:    "TEST_DISTRO_HOSTNAME",
			}

			ctrlClient, controlService := newCtrlStream(t, ctx)
			ctrlClient.sendErr = tc.ctrlStreamSendErr

			system, mock := testutils.MockSystem(t)

			if tc.getSystemErr {
				os.Remove(mock.Path("etc/os-release"))
			}

			mock.SetControlArg(testutils.ProStatusAttached)
			if tc.proStatusErr {
				mock.SetControlArg(testutils.ProStatusErr)
			}

			switch tc.proDetachErr {
			case detachOK:
			case detachAlreadyDetached:
				mock.SetControlArg(testutils.ProDetachErrAlreadyDetached)
			case detachErr:
				mock.SetControlArg(testutils.ProDetachErrGeneric)
			default:
				require.Fail(t, "Unknown enum value for detachResult", "Value: %d", tc.proDetachErr)
			}

			if tc.attachErr {
				mock.SetControlArg(testutils.ProAttachErr)
			}

			wslClient := setupWSLInstanceService(t, ctx, ctrlClient, system)

			errCh := make(chan error)
			go func() {
				_, err := wslClient.ApplyProToken(ctx, &wslserviceapi.ProAttachInfo{Token: tc.token})
				errCh <- err
			}()

			err := <-errCh
			if tc.wantErr {
				require.Error(t, err, "ProAttach call should return an error")
				return
			}
			require.NoError(t, err, "ProAttach call should return no error")

			got, err := controlService.recv()
			require.NoError(t, err, "ctrlClient should receive an info sent from the wslinstanceservice")
			require.Equal(t, wantSysInfo, got, "System info sent to agent does not match the expected one")
		})
	}
}

func TestApplyLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		emptyConfig bool
		enableErr   bool
		disableErr  bool

		wantErr bool
	}{
		// Enable
		"Success enabling":  {},
		"Success disabling": {emptyConfig: true},

		"Error enabling when landscape-config fails":            {enableErr: true, wantErr: true},
		"Error disabling when landscape-config --disable fails": {emptyConfig: true, disableErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			system, mock := testutils.MockSystem(t)

			if tc.enableErr {
				mock.SetControlArg(testutils.LandscapeEnableErr)
			}

			if tc.disableErr {
				mock.SetControlArg(testutils.LandscapeDisableErr)
			}

			ctrlClient, _ := newCtrlStream(t, ctx)
			wslClient := setupWSLInstanceService(t, ctx, ctrlClient, system)

			var config string
			if !tc.emptyConfig {
				config = "[hello]\nworld: true"
			}

			empty, err := wslClient.ApplyLandscapeConfig(ctx, &wslserviceapi.LandscapeConfig{Configuration: config})
			if tc.wantErr {
				require.Error(t, err, "ApplyLandscapeConfig call should return an error")
				return
			}
			require.NoError(t, err, "ApplyLandscapeConfig call should return no error")

			require.NotNil(t, empty, "ApplyLandscapeConfig should not return a nil response")
		})
	}
}

//nolint:revive // We've decided testing.T always preceedes the context.
func setupWSLInstanceService(t *testing.T, ctx context.Context, ctrlClient wslinstanceservice.ControlStreamClient, s system.System) wslserviceapi.WSLClient {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	sv := wslinstanceservice.New(s)
	server := sv.RegisterGRPCService(context.Background(), ctrlClient)

	var conf net.ListenConfig
	lis, err := conf.Listen(ctx, "tcp4", "localhost:")
	require.NoError(t, err, "Setup: WslInstance server could not listen")

	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	t.Logf("Serving WslInstanceService on %s", lis.Addr().String())

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Setup: could not dial WslInstance")

	t.Log("Client connected to WslInstanceService")

	return wslserviceapi.NewWSLClient(conn)
}

// controlStream mocks the GRPC calls without the need to set up an actual GRPC service.
type controlClient struct {
	ctx     context.Context
	ch      chan<- *agentapi.DistroInfo
	sendErr bool
}

type controlService struct {
	ctx context.Context
	ch  <-chan *agentapi.DistroInfo
}

//nolint:revive // We've decided testing.T always preceedes the context.
func newCtrlStream(t *testing.T, ctx context.Context) (*controlClient, *controlService) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	// Buffer size of 1 makes testing simpler as we can be more relaxed about ordering.
	ch := make(chan *agentapi.DistroInfo, 1)

	go func() {
		defer close(ch)
		<-ctx.Done()
	}()

	return &controlClient{ctx: ctx, ch: ch},
		&controlService{ctx: ctx, ch: ch}
}

// Send sends a distro info into the stream. Must be public to implement the interface.
func (s *controlClient) Send(info *agentapi.DistroInfo) error {
	if s.sendErr {
		return errors.New("test error")
	}

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case s.ch <- info:
		return nil
	}
}

// recv returns the latest info.
func (s *controlService) recv() (*agentapi.DistroInfo, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case info := <-s.ch:
		return info, nil
	}
}

func TestWithProMock(t *testing.T)             { testutils.ProMock(t) }
func TestWithLandscapeConfigMock(t *testing.T) { testutils.LandscapeConfigMock(t) }
func TestWithWslPathMock(t *testing.T)         { testutils.WslPathMock(t) }
func TestWithCmdExeMock(t *testing.T)          { testutils.CmdExeMock(t) }
