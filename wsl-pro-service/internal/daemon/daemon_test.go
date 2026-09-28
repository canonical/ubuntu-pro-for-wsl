package daemon_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/streams"
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
		missingCertsDir         bool
		missingCaCert           bool
		breakLandscapeConf      bool
		insecureAttributes      bool

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
		wantSystemError     bool
	}{
		"Success": {wantConnected: true},
		"Success with systemd notifier returning true": {notifierReturn: true, wantConnected: true},
		"Success with a broken Landscape config":       {breakLandscapeConf: true, wantConnected: true},

		// No connection:
		// These problems do not cause the agent to return error because it
		// keeps retrying the connection
		//
		// We instead check that a connection was/wasn't made with the agent, and that systemd was notified
		"No connection because the port file does not exist":         {breakPortFile: true, wantConnected: false},
		"No connection because the port file is empty":               {portFileEmpty: true, wantConnected: false},
		"No connection because the port file has a bad port":         {portFilePortNotNumber: true, wantConnected: false},
		"No connection because the port file has port 0":             {portFileZeroPort: true, wantConnected: false},
		"No connection because the port file has a negative port":    {portFileNegativePort: true, wantConnected: false},
		"No connection because there is no server":                   {dontServe: true},
		"No connection because there are no certificates":            {missingCertsDir: true, wantConnected: false},
		"No connection because cannot read root CA certificate file": {missingCaCert: true, wantConnected: false},

		// Errors
		"Error because the context is pre-cancelled":                                {precancelContext: true, wantSystemdNotReady: true, wantErr: true},
		"Error because the notifier returns an error":                               {notifierErr: true, wantErr: true},
		"Error because WindowsHostAddress returns an error":                         {breakWindowsHostAddress: true, wantErr: true},
		"Error and immediate termination because projected attributes are insecure": {insecureAttributes: true, wantErr: true, wantSystemError: true},
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

			if tc.missingCertsDir {
				require.NoError(t, os.RemoveAll(filepath.Join(publicDir, common.CertificatesDir)), "Setup: could not remove certificates")
			}

			if tc.missingCaCert {
				require.NoError(t, os.RemoveAll(filepath.Join(publicDir, common.CertificatesDir, common.RootCACertFileName)), "Setup: could not remove the root CA certificate file")
			}

			if tc.breakPortFile {
				require.NoError(t, os.Remove(filepath.Join(publicDir, common.ListeningPortFileName)), "Setup: could not remove port file")
			}

			if tc.breakLandscapeConf {
				require.NoError(t, os.RemoveAll(system.Path("/etc/landscape/client.conf")), "Setup: couldn't remove Landscape client conf to break tests")
				require.NoError(t, os.MkdirAll(system.Path("/etc/landscape/client.conf"), 0750), "Setup: couldn't create a directory to break Landscape client conf for tests")
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

			systemd := &SystemdSdNotifierMock{
				returns:   tc.notifierReturn,
				returnErr: tc.notifierErr,
			}

			var opts []daemon.Option
			opts = append(opts, daemon.WithSystemdNotifier(systemd.notify))
			if tc.insecureAttributes {
				mockReader := testutils.NewMockSecureReader(func(rootDir, targetPath string) ([]byte, error) {
					return nil, fmt.Errorf("refused %q: not strictly owned by root (uid 1000, gid 1000)", targetPath)
				})
				opts = append(opts, daemon.WithTestSecureReader(mockReader))
			} else {
				mockReader := testutils.NewMockSecureReader(nil)
				opts = append(opts, daemon.WithTestSecureReader(mockReader))
			}

			d, err := daemon.New(ctx, system, opts...)
			require.NoError(t, err, "New should return no error")

			if tc.precancelContext {
				cancel()
			}

			serveExit := make(chan error)
			go func() {
				serveExit <- d.Serve(&mockService{})
				close(serveExit)
			}()

			if tc.wantConnected {
				require.Eventually(t, func() bool {
					return systemd.gotState.Load() == "STATUS=Connected"
				}, 30*time.Second, time.Second, "Systemd never switched states to 'Connected'")

				require.Eventually(t, agent.Service.AllConnected, 30*time.Second, time.Second, "The daemon should have connected to the Windows Agent")

				require.Eventually(t, func() bool {
					conOk := len(agent.Service.Connect.History()) > 0
					proOk := len(agent.Service.ProAttachment.History()) > 0
					lpeOk := len(agent.Service.LandscapeConfig.History()) > 0
					return conOk && proOk && lpeOk
				}, 30*time.Second, time.Second, "The server should have been sent the Hello message on every stream")
			} else if tc.wantErr {
				select {
				case err := <-serveExit:
					require.Error(t, err, "Serve should have returned an error")
					if tc.wantSystemError {
						require.ErrorIs(t, err, streams.SystemError{}, "Serve should return a streams.SystemError")
					}
				case <-time.After(30 * time.Second):
					require.Fail(t, "Serve should have returned an error, but is still serving")
				}
			} else {
				// Not connected, but no return either: silent error and retrial
				require.Eventually(t, func() bool {
					return strings.HasPrefix(systemd.gotState.Load(), "STATUS=Not connected")
				}, 30*time.Second, time.Second, "Systemd never switched states to 'Not connected'")
			}

			d.Quit(ctx, false)

			if !tc.wantErr {
				select {
				case err := <-serveExit:
					require.NoError(t, err, "Serve() should have returned no error")
				case <-time.After(30 * time.Second):
					require.Fail(t, "Serve should have exited after calling Quit")
				}
			}

			if tc.wantSystemdNotReady {
				require.Zero(t, systemd.readyNotifications.Load(), "daemon should not have notified systemd")
			} else {
				require.EqualValues(t, 1, systemd.readyNotifications.Load(), "daemon should have notified systemd once")
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
		})
	}
}

func TestServe_FailsOnInsecureClientKey(t *testing.T) {
	t.Parallel()

	// An insecure client key is a non-transient error: connect() wraps it in a
	// streams.SystemError, serveOnce bubbles it up, and retryConfig.Run returns
	// it immediately without entering a backoff delay. The deadline therefore
	// only elapses on a regression (e.g. the error is no longer classified as a
	// SystemError and Serve enters the retry backoff instead). Bounding the wait
	// makes that regression fail fast with a clear message, and cancelling the
	// daemon through ctx lets the serveExit goroutine drain before the test ends.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	system, mock := testutils.MockSystem(t)
	publicDir := mock.DefaultPublicDir()
	agent := testutils.NewMockWindowsAgent(t, ctx, publicDir)
	defer agent.Stop()

	keyPath := filepath.Join(common.CertificatesDir, common.ClientsCertFilePrefix+common.KeySuffix)

	mockReader := testutils.NewMockSecureReader(func(rootDir, targetPath string) ([]byte, error) {
		if targetPath == keyPath {
			return nil, fmt.Errorf("refused %q: not strictly owned by root (uid 1000, gid 1000)", targetPath)
		}
		return os.ReadFile(filepath.Join(rootDir, targetPath))
	})

	d, err := daemon.New(ctx, system, daemon.WithTestSecureReader(mockReader))
	require.NoError(t, err)

	serveExit := make(chan error, 1)
	go func() { serveExit <- d.Serve(&mockService{}) }()

	select {
	case err := <-serveExit:
		require.Error(t, err)
		require.ErrorIs(t, err, streams.SystemError{}, "insecure client key must cause immediate SystemError termination")
		require.Contains(t, err.Error(), "not strictly owned by root")
	case <-ctx.Done():
		t.Fatalf("Serve did not terminate with a SystemError within 10s: %v", ctx.Err())
	}
}

func TestServe_FailsOnMissingClientKeyIsNotSystemError(t *testing.T) {
	t.Parallel()

	// Bound the two state waits below so that a regression that stops Serve from
	// reaching the retry path (or from exiting after Quit) fails fast instead of
	// hanging until the whole test binary times out. The happy paths complete in
	// a single connection attempt, so the deadline is a fail-safe, not a delay.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	system, mock := testutils.MockSystem(t)
	publicDir := mock.DefaultPublicDir()
	agent := testutils.NewMockWindowsAgent(t, ctx, publicDir)
	defer agent.Stop()

	keyPath := filepath.Join(common.CertificatesDir, common.ClientsCertFilePrefix+common.KeySuffix)

	mockReader := testutils.NewMockSecureReader(func(rootDir, targetPath string) ([]byte, error) {
		if targetPath == keyPath {
			return nil, os.ErrNotExist
		}
		return os.ReadFile(filepath.Join(rootDir, targetPath))
	})

	// Missing file is transient (agent not started yet): connect() returns a
	// non-SystemError, serveOnce reports success=false and the retry loop enters
	// onWait, which publishes "Not connected: waiting to retry" through the
	// systemd notifier. Intercept that status with a channel so the test waits
	// on the observable consequence of the missing key (rather than a fixed
	// wall-clock window).
	retrying := make(chan struct{}, 1)
	notifier := func(_ bool, state string) (bool, error) {
		if strings.Contains(state, "Not connected: waiting to retry") {
			select {
			case retrying <- struct{}{}:
			default:
			}
		}
		return false, nil
	}

	d, err := daemon.New(ctx, system,
		daemon.WithSystemdNotifier(notifier),
		daemon.WithTestSecureReader(mockReader),
	)
	require.NoError(t, err)

	serveExit := make(chan error, 1)
	go func() { serveExit <- d.Serve(&mockService{}) }()

	// Once the daemon publishes "not connected: waiting to retry", it has
	// classified the missing key as transient and is back in the retry loop.
	// Reaching the retry state is the deterministic, observable consequence of
	// the missing key. The only way Serve can exit at this point instead is a
	// bug (e.g. the missing key being misclassified as a SystemError), which
	// surfaces as an early receive below.
	select {
	case <-retrying:
		// Expected: the daemon is waiting to retry.
	case err := <-serveExit:
		require.Failf(t, "Serve should not have exited for missing key (transient)", "got %v", err)
	case <-ctx.Done():
		t.Fatalf("daemon did not reach the retry path within 10s: %v", ctx.Err())
	}

	// Quit cancels the daemon's graceful context and blocks until Serve returns.
	// A missing key must never surface as a SystemError.
	d.Quit(ctx, false)

	select {
	case err := <-serveExit:
		require.NotErrorIs(t, err, streams.SystemError{}, "missing key should not be a SystemError")
	case <-ctx.Done():
		t.Fatalf("Serve did not exit after Quit within deadline: %v", ctx.Err())
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

			systemd := &SystemdSdNotifierMock{
				returns: true,
			}

			mockReader := testutils.NewMockSecureReader(nil)

			d, err := daemon.New(ctx, system,
				daemon.WithSystemdNotifier(systemd.notify),
				daemon.WithTestSecureReader(mockReader),
			)
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

				require.Eventually(t, agent.Service.AllConnected, 10*time.Second, 500*time.Millisecond, "Daemon never connected to agent's service")
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

func TestRetryLogic(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		succeedWithoutRetries  bool
		actionError            error
		precancelled           bool
		cancelledBeforeMaxWait bool

		wantNoRetries       bool
		wantTooManyAttempts bool
		wantErr             bool
	}{
		"Without retries":                          {succeedWithoutRetries: true, wantNoRetries: true},
		"With the context pre-cancelled":           {precancelled: true, wantNoRetries: true},
		"With the context cancelled while waiting": {cancelledBeforeMaxWait: true},
		"When max attempts are exhausted":          {wantTooManyAttempts: true},

		"Error only when action errors": {actionError: errors.New("wanted error"), wantNoRetries: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			minWait := 10 * time.Millisecond
			maxWait := 7 * minWait
			var maxRetries uint8 = 8

			ctxTimeout := 10 * maxWait
			if tc.cancelledBeforeMaxWait {
				ctxTimeout = 3 * minWait
			}
			ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
			if tc.precancelled {
				cancel()
				<-ctx.Done() // Otherwise the test may finish before the context is trully cancelled.
			} else {
				defer cancel()
			}
			tryCount := 0
			tooManyAttempts := false
			rc := daemon.NewRetryConfig(minWait, maxWait, maxRetries)
			// All functions passed below run in the same goroutine, thus no need for
			// synchronisation.
			err := rc.Run(ctx, func() (bool, error) {
				return tc.succeedWithoutRetries, tc.actionError
			}, func(wait time.Duration) {
				if tc.wantNoRetries {
					require.LessOrEqual(t, tryCount, 1, "Unexpected Retry attempt")
				}
				tryCount++
			}, func() {
				if !tc.wantTooManyAttempts {
					require.Fail(t, "Unexpected too many retry attempts")
				}
				tooManyAttempts = true
			})
			if tc.wantErr {
				require.Error(t, err, "rc.Run() should fail with the supplied arguments")
			}
			if tc.wantNoRetries {
				require.LessOrEqual(t, tryCount, 1, "Action should be tried at most once")
			}
			require.Equal(t, tc.wantTooManyAttempts, tooManyAttempts, "Mismatched expectation about calling the too many attempts callback")
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

			systemd := &SystemdSdNotifierMock{returns: true}

			mockReader := testutils.NewMockSecureReader(nil)

			d, err := daemon.New(ctx, system,
				daemon.WithSystemdNotifier(systemd.notify),
				daemon.WithTestSecureReader(mockReader),
			)
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

				require.Eventually(t, agent.Service.AllConnected, 10*time.Second, 500*time.Millisecond, "Daemon never connected to agent's service")

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

			require.Eventually(t, agent.Service.AllConnected, 20*time.Second, 500*time.Millisecond, "Daemon never connected to agent's service")
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
