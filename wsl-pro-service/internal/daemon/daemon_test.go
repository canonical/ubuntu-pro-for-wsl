package daemon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sys, mock := testutils.MockSystem(t)

			if tc.breakWslPath {
				mock.SetControlArg(testutils.WslpathErr)
			}

			_, err := daemon.New(ctx, sys)
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
		breakWindowsHostAddress bool
		dontServe               bool

		// Break the port file in various ways
		breakPortFile         bool
		portFileEmpty         bool
		portFilePortNotNumber bool
		portFileZeroPort      bool
		portFileNegativePort  bool

		// Return values for the mock SystemdSdNotifier
		notifierReturn bool
		notifierErr    bool

		wantSystemdNotReady bool
		wantConnected       bool
		wantErr             bool
	}{
		"Success": {wantConnected: true},
		"Success with systemd notifier returning true": {notifierReturn: true, wantConnected: true},

		// No connection:
		// These problems do not cause the agent to return error because it
		// keeps retrying the connection
		//
		// We instead check that a connection was/wasn't made with the agent, and that systemd was notified
		"No connection because the port file does not exist":      {breakPortFile: true, wantSystemdNotReady: true, wantConnected: false},
		"No connection because the port file is empty":            {portFileEmpty: true, wantSystemdNotReady: true, wantConnected: false},
		"No connection because the port file has a bad port":      {portFilePortNotNumber: true, wantSystemdNotReady: true, wantConnected: false},
		"No connection because the port file has port 0":          {portFileZeroPort: true, wantSystemdNotReady: true, wantConnected: false},
		"No connection because the port file has a negative port": {portFileNegativePort: true, wantSystemdNotReady: true, wantConnected: false},
		"No connection because there is no server":                {dontServe: true},

		// Errors
		"Error because the context is pre-cancelled":        {precancelContext: true, wantSystemdNotReady: true, wantErr: true},
		"Error because the notifier returns an error":       {notifierErr: true, wantErr: true},
		"Error because WindowsHostAddress returns an error": {breakWindowsHostAddress: true, wantSystemdNotReady: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			publicDir := mock.DefaultPublicDir()
			agent := testutils.NewMockWindowsAgent(t, ctx, publicDir)
			defer agent.Stop()

			if tc.breakPortFile {
				err := os.RemoveAll(publicDir)
				require.NoError(t, err, "Setup: could not remove port file")
			}

			if tc.breakWindowsHostAddress {
				mock.SetControlArg(testutils.WslInfoErr)
			}

			portFile := filepath.Join(publicDir, common.ListeningPortFileName)
			if tc.portFileEmpty {
				require.NoError(t, os.WriteFile(portFile, []byte{}, 0600), "Setup: could not overwrite port file")
			}
			if tc.portFilePortNotNumber {
				require.NoError(t, os.WriteFile(portFile, []byte("127.0.0.1:portyMcPortface"), 0600), "Setup: could not overwrite port file")
			}
			if tc.portFileZeroPort {
				require.NoError(t, os.WriteFile(portFile, []byte("127.0.0.1:0"), 0600), "Setup: could not overwrite port file")
			}
			if tc.portFileNegativePort {
				require.NoError(t, os.WriteFile(portFile, []byte("127.0.0.1:-5"), 0600), "Setup: could not overwrite port file")
			}
			if tc.dontServe {
				addr := agent.Listener.Addr().String()
				agent.Stop()
				require.NoError(t, os.WriteFile(portFile, []byte(addr), 0600), "Setup: could not overwrite port file")
			}

			systemd := SystemdSdNotifierMock{
				returns:   tc.notifierReturn,
				returnErr: tc.notifierErr,
			}

			d, err := daemon.New(ctx, system, daemon.WithSystemdNotifier(systemd.notify))
			require.NoError(t, err, "New should return no error")

			if tc.precancelContext {
				cancel()
			}

			time.AfterFunc(20*time.Second, func() { d.Quit(ctx, true) })

			err = d.Serve(&mockService{})
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

			if tc.dontServe {
				return // Nothing to assert server-side
			}

			if !tc.wantConnected {
				require.Zero(t, agent.Service.Connect.NConnections(), "daemon should not have connected to the agent (connected stream)")
				require.Zero(t, agent.Service.ProAttachment.NConnections(), "daemon should not have connected to the agent (pro attach stream)")
				require.Zero(t, agent.Service.LandscapeConfig.NConnections(), "daemon should not have connected to the agent (landscape config stream)")
				return
			}

			require.NotZero(t, agent.Service.Connect.NConnections(), "daemon should have connected to the agent (connected stream)")
			require.NotZero(t, agent.Service.ProAttachment.NConnections(), "daemon should have connected to the agent (pro attach stream)")
			require.NotZero(t, agent.Service.LandscapeConfig.NConnections(), "daemon should have connected to the agent (landscape config stream)")
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
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			publicDir := mock.DefaultPublicDir()
			agent := testutils.NewMockWindowsAgent(t, ctx, publicDir)

			systemd := SystemdSdNotifierMock{
				returns: true,
			}

			d, err := daemon.New(ctx, system, daemon.WithSystemdNotifier(systemd.notify))
			require.NoError(t, err, "New should return no error")

			if tc.quitBeforeServe {
				d.Quit(ctx, tc.quitForcefully)
			}

			serveExit := make(chan error)
			go func() {
				serveExit <- d.Serve(&mockService{})
				close(serveExit)
			}()

			if !tc.quitBeforeServe {
				// Wait for the server to start
				require.Eventually(t, func() bool {
					return systemd.readyNotifications.Load() > 0
				}, 20*time.Second, 100*time.Millisecond, "Systemd should have been notified")

				const wantState = "STATUS=Connected"
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == wantState
				}, 20*time.Second, time.Second, "Systemd state should have been set to %q ", wantState)

				require.False(t, systemd.gotUnsetEnvironment.Load(), "Unexpected value sent by Daemon to systemd notifier's unsetEnvironment")

				require.Eventually(t, agent.Service.AllConnected, 10*time.Second, 100*time.Millisecond, "Daemon never connected to agent's service")
			}

			d.Quit(ctx, tc.quitForcefully)

			select {
			case <-time.After(20 * time.Second):
				require.Fail(t, "Serve should have exited after calling Quit")
			case err = <-serveExit:
			}

			if tc.wantErr {
				require.Error(t, err, "Serve should have returned an error")
				require.LessOrEqual(t, systemd.readyNotifications.Load(), int32(1), "Systemd notifier should have been notified at most once")
				return
			}
			require.NoError(t, err, "Serve should have returned no errors")

			require.Eventually(t, func() bool { return !agent.Service.AnyConnected() },
				10*time.Second, 100*time.Millisecond, "Service should have disconnected from the agent")

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
		firstConnectionLong      bool
	}{
		"Success connecting after failing to connect":                     {},
		"Success connecting after previous connection dropped":            {firstConnectionSuccesful: true},
		"Success connecting after previous long-lived connection dropped": {firstConnectionLong: true, firstConnectionSuccesful: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)
			publicDir := mock.DefaultPublicDir()

			systemd := SystemdSdNotifierMock{returns: true}

			d, err := daemon.New(ctx, system, daemon.WithSystemdNotifier(systemd.notify))
			require.NoError(t, err, "New should return no error")

			defer d.Quit(ctx, true)

			var agent *testutils.MockWindowsAgent
			if tc.firstConnectionSuccesful {
				agent = testutils.NewMockWindowsAgent(t, ctx, publicDir)
				defer agent.Stop()
			}

			//nolint:errcheck // We don't really care
			go d.Serve(&mockService{})

			const maxTimeout = 60 * time.Second

			if tc.firstConnectionSuccesful {
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == "STATUS=Connected"
				}, maxTimeout, time.Second, "Service should have set systemd state to Connected")

				require.Eventually(t, agent.Service.AllConnected, 10*time.Second, 100*time.Millisecond, "Daemon never connected to agent's service")

				if tc.firstConnectionLong {
					// "Long-lived" means longer than a minute
					time.Sleep(65 * time.Second)
				}

				agent.Stop()
			} else {
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == "STATUS=Not connected: waiting to retry"
				}, maxTimeout, 100*time.Millisecond, "State should have been set to 'Not connected: waiting to retry'")
			}

			agent = testutils.NewMockWindowsAgent(t, ctx, publicDir)
			defer agent.Stop()

			require.Eventually(t, agent.Service.AllConnected, 20*time.Second, 100*time.Millisecond, "Daemon never connected to agent's service")
			require.EqualValues(t, 1, systemd.readyNotifications.Load(), "Service should have notified systemd after connecting to the control stream")
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

type mockService struct{}

func (s *mockService) ApplyProToken(ctx context.Context, msg *agentapi.ProAttachCmd) error {
	return nil
}

func (s *mockService) ApplyLandscapeConfig(ctx context.Context, msg *agentapi.LandscapeConfigCmd) error {
	return nil
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T) { testutils.WslInfoMock(t) }
func TestWithCmdExeMock(t *testing.T)  { testutils.CmdExeMock(t) }
