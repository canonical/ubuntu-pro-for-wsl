package daemon_test

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

func TestNew(t *testing.T) {
	t.Parallel()

	type dataFileState int

	const (
		dataFileGood dataFileState = iota
		dataFileUnreadable
		dataFileNotExist
		dataFileEmpty
		dataFileBadSyntax
		dataFileBadData
	)

	testCases := map[string]struct {
		portFile   dataFileState
		resolvFile dataFileState

		agentDoesntRecv   bool
		agentSendsNoPort  bool
		agentSendsBadPort bool

		precancelContext bool

		wantErr bool
	}{
		"Success": {},

		// Logic error: triggers a hard-to-exercise error when asyncronously dialing the control stream
		"Error because of context cancelled": {precancelContext: true, wantErr: true},

		// Port file errors
		"Error because port file does not exist":             {portFile: dataFileNotExist, wantErr: true},
		"Error because of unreadable port file":              {portFile: dataFileUnreadable, wantErr: true},
		"Error because of empty port file":                   {portFile: dataFileEmpty, wantErr: true},
		"Error because of port file with invalid contents":   {portFile: dataFileBadSyntax, wantErr: true},
		"Error because of port file contains the wrong port": {portFile: dataFileBadData, wantErr: true},

		// Resolv.conf errors
		"Error because resolv.conf does not exist":                    {resolvFile: dataFileNotExist, wantErr: true},
		"Error because of unreadable resolv.conf":                     {resolvFile: dataFileUnreadable, wantErr: true},
		"Error because of empty resolv.conf":                          {resolvFile: dataFileEmpty, wantErr: true},
		"Error because of resolv.conf with invalid contents":          {resolvFile: dataFileBadSyntax, wantErr: true},
		"Error because of resolv.conf contains an invalid nameserver": {resolvFile: dataFileBadData, wantErr: true},

		// Agent errors
		"Error because of Agent never receives":     {agentDoesntRecv: true, wantErr: true},
		"Error because of Agent never sends a port": {agentSendsNoPort: true, wantErr: true},
		"Error because of Agent sends port :0":      {agentSendsBadPort: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystemInfo(t)

			var agentArgs []testutils.AgentOption
			if tc.agentDoesntRecv {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeReceivingInfo())
			} else if tc.agentSendsNoPort {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeSendingPort())
			} else if tc.agentSendsBadPort {
				agentArgs = append(agentArgs, testutils.WithSendBadPort())
			}

			portFile := mock.DefaultAddrFile()
			testutils.MockWindowsAgent(t, ctx, portFile, agentArgs...)

			switch tc.portFile {
			case dataFileGood:
			case dataFileNotExist:
				err := os.Remove(portFile)
				require.NoError(t, err, "Setup: could not remove port file")
			case dataFileUnreadable:
				err := os.Remove(portFile)
				require.NoError(t, err, "Setup: could not remove port file")
				err = os.Mkdir(portFile, 0600)
				require.NoError(t, err, "Setup: could not create directory where port file should be")
			case dataFileEmpty:
				f, err := os.Create(portFile)
				require.NoError(t, err, "Setup: failed to create empty port file")
				f.Close()
			case dataFileBadSyntax:
				err := os.WriteFile(portFile, []byte("This text is not a valid IP address"), 0600)
				require.NoError(t, err, "Setup: failed to create port file with invalid contents")
			case dataFileBadData:
				lis, err := net.Listen("tcp4", "localhost:")
				require.NoError(t, err, "Setup: could not reserve an IP address to mess with port file")
				wrongAddr := lis.Addr().String()

				err = os.WriteFile(portFile, []byte(wrongAddr), 0600)
				require.NoError(t, err, "Setup: failed to create port file with misleading contents")

				err = lis.Close()
				require.NoError(t, err, "Setup: failed to close port file used to select wrong port")
			default:
				require.Fail(t, "Test setup error", "Unexpected enum value %d for portFile state", tc.portFile)
			}

			resolvConf := mock.Path("etc/resolv.conf")
			switch tc.resolvFile {
			case dataFileGood:
				copyFile(t, "testdata/resolv.conf", resolvConf)
			case dataFileNotExist:
				err := os.Remove(resolvConf)
				require.NoError(t, err, "Setup: could not remove resolv.conf file")
			case dataFileUnreadable:
				err := os.Remove(resolvConf)
				require.NoError(t, err, "Setup: could not remove resolv.conf file")
				err = os.Mkdir(resolvConf, 0600)
				require.NoError(t, err, "Setup: could not write /etc/resolv.conf/ directory")
			case dataFileEmpty:
				f, err := os.Create(resolvConf)
				require.NoError(t, err, "Setup: could not create empty resolv.conf file")
				f.Close()
			case dataFileBadSyntax:
				err := os.WriteFile(resolvConf, []byte("This text is not\nvalid for a resolv.conf file"), 0600)
				require.NoError(t, err, "Setup: could not create resolv.conf file with invalid contents")
			case dataFileBadData:
				copyFile(t, "testdata/bad_resolv.conf", resolvConf)
			default:
				require.Fail(t, "Test setup error", "Unexpected enum value %d for resolv.conf file state", tc.portFile)
			}

			var regCount int
			countRegistrations := func(context.Context, wslinstanceservice.ControlStreamClient) *grpc.Server {
				regCount++
				return nil
			}

			if tc.precancelContext {
				cancel()
			}

			_, err := daemon.New(
				ctx,
				system,
				portFile,
				countRegistrations,
			)
			if tc.wantErr {
				require.Error(t, err, "New should have errored out but hasn't")
				return
			}

			require.NoError(t, err, "New() should have returned no error")
			require.Equal(t, 1, regCount, "daemon should register GRPC services only once")
		})
	}
}

func TestServeAndQuit(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		precancelContext bool

		quitBeforeServe bool
		quiteForcefully bool
		quitTwice       bool

		// Return values for the mock SystemdSdNotifier to return
		notifierReturn bool
		notifierErr    bool

		// Return value of (Daemon).Serve
		wantErr bool
	}{
		"Success with graceful quit":            {notifierReturn: true},
		"Success with forceful quit":            {notifierReturn: true, quiteForcefully: true},
		"Success with double quit":              {notifierReturn: true, quitTwice: true},
		"Success with notifier returning false": {notifierReturn: false},

		"Error due to cancelled context":       {precancelContext: true, wantErr: true},
		"Error due to quitting before serving": {quitBeforeServe: true, wantErr: true},
		"Error with notifier returning error":  {notifierErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			system, mock := testutils.MockSystemInfo(t)

			portFile := mock.DefaultAddrFile()
			testutils.MockWindowsAgent(t, ctx, portFile)

			registerer := func(ctx context.Context, ctrl wslinstanceservice.ControlStreamClient) *grpc.Server {
				// No need for a real GRPC service
				return grpc.NewServer()
			}

			systemd := SystemdSdNotifierMock{
				returns:   tc.notifierReturn,
				returnErr: tc.notifierErr,
			}

			copyFile(t, "testdata/resolv.conf", mock.Path("etc/resolv.conf"))

			d, err := daemon.New(ctx,
				system,
				portFile,
				registerer,
				daemon.WithSystemdNotifier(systemd.notify),
			)
			require.NoError(t, err, "Setup: daemon.New should return no errors")

			serveExit := make(chan error)
			go func() {
				serveCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				if tc.precancelContext {
					cancel()
				}
				if tc.quitBeforeServe {
					d.Quit(ctx, tc.quiteForcefully)
				}

				serveExit <- d.Serve(serveCtx)
				close(serveExit)
			}()

			// Wait for the server to start
			time.Sleep(100 * time.Millisecond)

			d.Quit(ctx, tc.quiteForcefully)

			if tc.wantErr {
				require.Error(t, <-serveExit, "Serve should have returned an error")
				require.LessOrEqual(t, systemd.nNotifications, 1, "Systemd notifier should have been notified at most once")
				return
			}
			require.NoError(t, <-serveExit, "Serve should have returned no errors")

			require.Equal(t, 1, systemd.nNotifications, "Systemd notifier should have been notified only once")
			require.False(t, systemd.gotUnsetEnvironment, "Unexpected value sent by Daemon to systemd notifier's unsetEnvironment")
			require.Equal(t, "READY=1", systemd.gotState, "Unexpected value sent by Daemon to systemd notifier's state")

			if !tc.quitTwice {
				return
			}

			d.Quit(ctx, tc.quiteForcefully)
		})
	}
}

type SystemdSdNotifierMock struct {
	returns   bool
	returnErr bool

	gotUnsetEnvironment bool
	gotState            string
	nNotifications      int
}

func (s *SystemdSdNotifierMock) notify(unsetEnvironment bool, state string) (bool, error) {
	s.nNotifications++
	s.gotUnsetEnvironment = unsetEnvironment
	s.gotState = state

	if s.returnErr {
		return s.returns, errors.New("mock error")
	}
	return s.returns, nil
}

// copyFile copies file src into dst.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	out, err := os.ReadFile(src)
	require.NoErrorf(t, err, "Setup: could not read resolv.conf file at %q", src)

	err = os.WriteFile(dst, out, 0600)
	require.NoErrorf(t, err, "Setup: could not write resolv.conf file at %q", dst)
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
