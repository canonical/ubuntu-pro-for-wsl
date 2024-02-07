package daemon_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/wslinstanceservice"
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

	testCases := map[string]struct {
		breakWslPath bool

		wantErr bool
	}{
		"Success":                          {},
		"Error when WslPath returns error": {breakWslPath: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sys, mock := testutils.MockSystem(t)

			if tc.breakWslPath {
				mock.SetControlArg(testutils.WslpathErr)
			}

			_, err := daemon.New(ctx, nil, sys)
			if tc.wantErr {
				require.Error(t, err, "New should return an error")
				return
			}

			require.NoError(t, err, "New should return no error")
		})
	}
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

		wantSystemdNotReady      bool
		wantConnectControlStream bool
		wantErr                  bool
	}{
		"Success": {wantConnectControlStream: true},
		"Success with systemd notifier returning true": {notifierReturn: true, wantConnectControlStream: true},

		// No connection:
		// These problems do not cause the agent to return error because it
		// keeps retrying the connection
		//
		// We instead chech that a connection was/wasn't made with the agent, and that systemd was notified
		"No connection because port file does not exist": {breakPortFile: true},
		"No connection because of faulty agent":          {agentDoesntRecv: true, wantConnectControlStream: true},

		// Errors
		"Error because of notifier returning error":      {notifierErr: true, wantErr: true},
		"Error because WindowsHostAddress returns error": {breakWindowsHostAddress: true, wantErr: true},
		"Error because of context cancelled":             {precancelContext: true, wantSystemdNotReady: true, wantErr: true},
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

			systemd := SystemdSdNotifierMock{
				returns:   tc.notifierReturn,
				returnErr: tc.notifierErr,
			}

			d, err := daemon.New(
				ctx,
				registerService,
				system,
				daemon.WithSystemdNotifier(systemd.notify),
			)
			require.NoError(t, err, "New should return no error")

			if tc.precancelContext {
				cancel()
			}

			time.AfterFunc(10*time.Second, func() { d.Quit(ctx, true) })

			err = d.Serve()
			if tc.wantErr {
				require.Error(t, err, "Serve() should have returned an error")
			} else {
				require.NoError(t, err, "Serve() should have returned no error")
			}

			if tc.wantSystemdNotReady {
				require.Zero(t, systemd.readyNotifications.Load(), "daemon should not have notified systemd")
			} else {
				require.Equal(t, int32(1), systemd.readyNotifications.Load(), "daemon should have notified systemd once")
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

			d, err := daemon.New(ctx,
				registerer,
				system,
				daemon.WithSystemdNotifier(systemd.notify),
			)
			require.NoError(t, err, "New should return no error")

			serveExit := make(chan error)
			go func() {
				if tc.quitBeforeServe {
					d.Quit(ctx, tc.quitForcefully)
				}

				serveExit <- d.Serve()
				close(serveExit)
			}()

			if !tc.quitBeforeServe {
				// Wait for the server to start
				require.Eventually(t, func() bool {
					return systemd.readyNotifications.Load() > 0
				}, 10*time.Second, 100*time.Millisecond, "Systemd should have been notified")

				const wantState = "STATUS=Serving"
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == wantState
				}, 60*time.Second, time.Second, "Systemd state should have been set to %q ", wantState)

				require.False(t, systemd.gotUnsetEnvironment.Load(), "Unexpected value sent by Daemon to systemd notifier's unsetEnvironment")
			}

			d.Quit(ctx, tc.quitForcefully)

			if tc.wantErr {
				require.Error(t, <-serveExit, "Serve should have returned an error")
				require.LessOrEqual(t, systemd.readyNotifications.Load(), int32(1), "Systemd notifier should have been notified at most once")
				return
			}
			require.NoError(t, <-serveExit, "Serve should have returned no errors")

			require.Equal(t, int32(1), systemd.readyNotifications.Load(), "Systemd notifier should have been notified exactly once")
			require.False(t, systemd.gotUnsetEnvironment.Load(), "Unexpected value sent by Daemon to systemd notifier's unsetEnvironment")
			require.Equal(t, "STATUS=Stopped", systemd.gotState.Load(), "Unexpected value sent by Daemon to systemd notifier's state")

			if !tc.quitTwice {
				return
			}

			d.Quit(ctx, tc.quitForcefully)
		})
	}
}

func TestReconnection(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		firstConnectionSuccesful bool
	}{
		"Success connecting after failing to connect":          {},
		"Success connecting after previous connection dropped": {firstConnectionSuccesful: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			portFile := mock.DefaultAddrFile()

			registerer := func(ctx context.Context, ctrl wslinstanceservice.ControlStreamClient) *grpc.Server {
				// No need for a real GRPC service
				return grpc.NewServer()
			}

			systemd := SystemdSdNotifierMock{returns: true}

			d, err := daemon.New(ctx,
				registerer,
				system,
				daemon.WithSystemdNotifier(systemd.notify),
			)
			require.NoError(t, err, "New should return no error")

			defer d.Quit(ctx, true)

			var server *grpc.Server
			var agentData *testutils.MockAgentData
			if tc.firstConnectionSuccesful {
				server, agentData = testutils.MockWindowsAgent(t, ctx, portFile)
				defer server.Stop()
			}

			//nolint:errcheck // We don't really care
			go d.Serve()

			const maxTimeout = 60 * time.Second

			if tc.firstConnectionSuccesful {
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == "STATUS=Serving"
				}, maxTimeout, time.Second, "Service should have set systemd state to Serving")

				require.Equal(t, int32(1), agentData.ConnectionCount.Load(), "Service should have connected to the control stream")
				server.Stop()

				// Avoid a race where the portfile is not removed until after the next server starts
				require.Eventually(t, func() bool {
					_, err := os.Stat(portFile)
					return errors.Is(err, fs.ErrNotExist)
				}, 20*time.Second, 100*time.Millisecond, "Stopping the Windows-Agent mock server should remove the port file")
			} else {
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == "STATUS=Not serving: waiting to retry"
				}, maxTimeout, 100*time.Millisecond, "State should have been set to 'Not serving'")
			}

			server, agentData = testutils.MockWindowsAgent(t, ctx, portFile)
			defer server.Stop()

			require.Eventually(t, func() bool {
				return agentData.BackConnectionCount.Load() != 0
			}, time.Minute, time.Second, "Service should eventually connect to the agent")

			require.Equal(t, int32(1), systemd.readyNotifications.Load(), "Service should have notified systemd after connecting to the control stream")
			require.Equal(t, int32(1), agentData.ConnectionCount.Load(), "Service should have connected to the control stream")
		})
	}
}

type SystemdSdNotifierMock struct {
	returns   bool
	returnErr bool

	gotUnsetEnvironment atomic.Bool
	gotState            atomicString
	readyNotifications  atomic.Int32
}

func (s *SystemdSdNotifierMock) notify(unsetEnvironment bool, state string) (bool, error) {
	s.gotUnsetEnvironment.Store(unsetEnvironment)
	s.gotState.Store(state)

	if strings.Contains(state, "READY=1") {
		s.readyNotifications.Add(1)
	}

	if s.returnErr {
		return s.returns, errors.New("mock error")
	}
	return s.returns, nil
}

type atomicString struct {
	atomic.Value
}

func (s *atomicString) Store(str string) {
	s.Value.Store(str)
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
