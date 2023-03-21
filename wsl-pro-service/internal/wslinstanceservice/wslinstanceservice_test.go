package wslinstanceservice_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestProAttach(t *testing.T) {
	t.Parallel()

	type detachResult int
	const (
		detachOK detachResult = iota
		detachErrNoReason
		detachErrAlreadyDetached
		detachErrOtherReason
		detachErrCannotParseReason
	)

	testCases := map[string]struct {
		proStatusErr      bool
		getSystemInfoErr  bool
		detachResult      detachResult
		attachErr         bool
		ctrlStreamSendErr bool

		wantErr bool
	}{
		"success on attached machine": {},
		"success on detached machine": {detachResult: detachErrAlreadyDetached},

		// Pro status errors
		"Error calling pro status":  {proStatusErr: true, wantErr: true},
		"Error getting system info": {getSystemInfoErr: true, wantErr: true},

		// Detach errors
		"Error detaching pro, reason can be parsed":    {detachResult: detachErrOtherReason, wantErr: true},
		"Error detaching pro, reason cannot be parsed": {detachResult: detachErrCannotParseReason, wantErr: true},
		"Error detaching pro, no reasons specified":    {detachResult: detachErrNoReason, wantErr: true},

		// Other errors
		"Error calling pro attach":         {attachErr: true, wantErr: true},
		"Error cannot send info to stream": {ctrlStreamSendErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Hour)
			defer cancel()

			wantSysInfo := &agentapi.DistroInfo{
				WslName:     "TEST_DISTRO",
				Id:          "ubuntu",
				VersionId:   "22.04",
				PrettyName:  "Ubuntu 22.04.1 LTS",
				ProAttached: true,
			}

			ctrlClient, controlService := newCtrlStream(t, ctx)
			ctrlClient.sendErr = tc.ctrlStreamSendErr

			const wantToken = "1000"

			var args []wslinstanceservice.Option

			// Setting up system info
			{
				info, err := wantSysInfo, error(nil)
				if tc.getSystemInfoErr {
					info, err = nil, errors.New("test error")
				}
				args = append(args, wslinstanceservice.WithGetSystemInfo(info, err))
			}

			// Setting up pro status
			{
				attached, err := true, error(nil)
				if tc.proStatusErr {
					attached, err = false, errors.New("test error")
				}
				args = append(args, wslinstanceservice.WithProStatus(attached, err))
			}

			// Setting up pro detach
			{
				out, err := "", error(nil)
				switch tc.detachResult {
				case detachOK:
				case detachErrNoReason:
					out = "{}"
					err = errors.New("test error")
				case detachErrAlreadyDetached:
					out = `{"errors": [{"message_code": "unattached", "message": "test error"}]}`
					err = errors.New("test error")
				case detachErrOtherReason:
					out = `{"errors": [{"message_code": "test", "message": "test error"}]}`
					err = errors.New("test error")
				case detachErrCannotParseReason:
					out = "This is not valid JSON"
					err = errors.New("test error")
				default:
					require.Fail(t, "Unknown enum value for detachResult", "Value: %d", tc.detachResult)
				}
				args = append(args, wslinstanceservice.WithProDetach(out, err))
			}

			// Setting up pro attach
			{
				out, err := "", error(nil)
				if tc.attachErr {
					out, err = `test error`, errors.New("test error")
				}
				args = append(args, wslinstanceservice.WithProAttach(func(ctx context.Context, token string) ([]byte, error) {
					require.Equal(t, wantToken, token, "Called attach pro with the wrong token")
					return []byte(out), err
				}))
			}

			wslClient := setupWSLInstanceService(t, ctx, ctrlClient, args...)

			errCh := make(chan error)
			go func() {
				_, err := wslClient.ProAttach(ctx, &wslserviceapi.AttachInfo{Token: wantToken})
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

//nolint:revive // We've decided testing.T always preceedes the context.
func setupWSLInstanceService(t *testing.T, ctx context.Context, ctrlClient wslinstanceservice.ControlStreamClient, args ...wslinstanceservice.Option) wslserviceapi.WSLClient {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	sv := wslinstanceservice.New(args...)
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
