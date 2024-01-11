package daemon_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"sync/atomic"
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

func TestServe(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		precancelContext        bool
		breakPortFile           bool
		breakWindowsHostAddress bool

		// Breaking the agent
		agentDoesntRecv   bool
		agentSendsNoPort  bool
		agentSendsBadPort bool

		// Return values for the mock SystemdSdNotifier
		notifierReturn bool
		notifierErr    bool

		wantSystemdNotified      bool
		wantConnectControlStream bool
		wantErr                  bool
	}{
		"Success": {wantConnectControlStream: true, wantSystemdNotified: true},
		"Success with systemd notifier returning true": {notifierReturn: true, wantConnectControlStream: true, wantSystemdNotified: true},

		// No connection:
		// These problems do not cause the agent to return error because it
		// keeps retrying the connection
		//
		// We instead chech that a connection was/wasn't made with the agent, and that systemd was notified
		"No connection because port file does not exist":    {breakPortFile: true},
		"No connection because of faulty agent":             {agentDoesntRecv: true, wantConnectControlStream: true},
		"No connection because of notifier returning error": {notifierErr: true, wantConnectControlStream: true, wantSystemdNotified: true},

		// Errors
		"Error because WindowsHostAddress returns error": {breakWindowsHostAddress: true, wantErr: true},
		"Error because of context cancelled":             {precancelContext: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			var agentArgs []testutils.AgentOption
			if tc.agentDoesntRecv {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeReceivingInfo())
			} else if tc.agentSendsNoPort {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeSendingPort())
			} else if tc.agentSendsBadPort {
				agentArgs = append(agentArgs, testutils.WithSendBadPort())
			}

			portFile := mock.DefaultAddrFile()
			_, agentMetaData := testutils.MockWindowsAgent(t, ctx, portFile, agentArgs...)

			if tc.breakPortFile {
				err := os.Remove(portFile)
				require.NoError(t, err, "Setup: could not remove port file")
			}

			if tc.breakWindowsHostAddress {
				mock.SetControlArg(testutils.WslInfoErr)
			}

			registerService := func(context.Context, wslinstanceservice.ControlStreamClient) *grpc.Server {
				// No need for an actual service
				return grpc.NewServer()
			}

			if tc.precancelContext {
				cancel()
			}

			systemd := SystemdSdNotifierMock{
				returns:   tc.notifierReturn,
				returnErr: tc.notifierErr,
			}

			d := daemon.New(
				ctx,
				portFile,
				registerService,
				system,
				daemon.WithSystemdNotifier(systemd.notify),
			)

			time.AfterFunc(10*time.Second, func() { d.Quit(ctx, true) })

			err := d.Serve()
			if tc.wantErr {
				require.Error(t, err, "Serve() should have returned an error")
			} else {
				require.NoError(t, err, "Serve() should have returned no error")
			}

			if tc.wantSystemdNotified {
				// NotZero rather than 1 because if the notification fails, it'll be retried every time
				require.NotZero(t, systemd.nNotifications.Load(), "daemon should have notified systemd")
			} else {
				require.Zero(t, systemd.nNotifications.Load(), "daemon should not have notified systemd")
			}

			if tc.wantConnectControlStream {
				require.NotZero(t, agentMetaData.ConnectionCount.Load(), "daemon should have succefully connected to the agent")
			} else {
				require.Zero(t, agentMetaData.ConnectionCount.Load(), "daemon should not have connected to the agent")
			}
		})
	}
}

func TestServeAndQuit(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		quitBeforeServe bool
		quitForcefully  bool
		quitTwice       bool

		// Return value of (Daemon).Serve
		wantErr bool
	}{
		"Success with graceful quit": {},
		"Success with forceful quit": {quitForcefully: true},
		"Success with double quit":   {quitTwice: true},

		"Error due to quitting before serving": {quitBeforeServe: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			portFile := mock.DefaultAddrFile()
			testutils.MockWindowsAgent(t, ctx, portFile)

			registerer := func(ctx context.Context, ctrl wslinstanceservice.ControlStreamClient) *grpc.Server {
				// No need for a real GRPC service
				return grpc.NewServer()
			}

			systemd := SystemdSdNotifierMock{
				returns: true,
			}

			d := daemon.New(ctx,
				portFile,
				registerer,
				system,
				daemon.WithSystemdNotifier(systemd.notify),
			)

			serveExit := make(chan error)
			go func() {
				if tc.quitBeforeServe {
					d.Quit(ctx, tc.quitForcefully)
				}

				serveExit <- d.Serve()
				close(serveExit)
			}()

			// Wait for the server to start
			time.Sleep(100 * time.Millisecond)

			d.Quit(ctx, tc.quitForcefully)

			if tc.wantErr {
				require.Error(t, <-serveExit, "Serve should have returned an error")
				require.LessOrEqual(t, systemd.nNotifications.Load(), int32(1), "Systemd notifier should have been notified at most once")
				return
			}
			require.NoError(t, <-serveExit, "Serve should have returned no errors")

			require.Equal(t, int32(1), systemd.nNotifications.Load(), "Systemd notifier should have been notified only once")
			require.False(t, systemd.gotUnsetEnvironment.Load(), "Unexpected value sent by Daemon to systemd notifier's unsetEnvironment")
			require.Equal(t, "READY=1", systemd.gotState.Load(), "Unexpected value sent by Daemon to systemd notifier's state")

			if !tc.quitTwice {
				return
			}

			d.Quit(ctx, tc.quitForcefully)
		})
	}
}

type SystemdSdNotifierMock struct {
	returns   bool
	returnErr bool

	gotUnsetEnvironment atomic.Bool
	gotState            atomicString
	nNotifications      atomic.Int32
}

func (s *SystemdSdNotifierMock) notify(unsetEnvironment bool, state string) (bool, error) {
	s.nNotifications.Add(1)
	s.gotUnsetEnvironment.Store(unsetEnvironment)
	s.gotState.Store(state)

	if s.returnErr {
		return s.returns, errors.New("mock error")
	}
	return s.returns, nil
}

type atomicString struct {
	atomic.Value
}

func (s *atomicString) Load() string {
	str, ok := s.Value.Load().(string)
	if !ok {
		return ""
	}
	return str
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T) { testutils.WslInfoMock(t) }
func TestWithCmdExeMock(t *testing.T)  { testutils.CmdExeMock(t) }
