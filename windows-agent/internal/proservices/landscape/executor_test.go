package landscape_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/landscape/landscapemockservice"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

const (
	testAppx       = "CanonicalGroupLimited.Ubuntu22.04LTS" // The name of the Appx
	testDistroAppx = "Ubuntu-22.04"                         // The name used in `wsl --install <DISTRO>`
)

func TestAssignHost(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		confErr bool
		wantErr bool
	}{
		"Success ": {},

		"Error when config returns an error": {confErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{},
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.confErr {
						testBed.conf.setLandscapeUIDErr = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_AssignHost_{AssignHost: &landscapeapi.Command_AssignHost{Uid: "HostUID123"}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = time.Second
					if tc.wantErr {
						time.Sleep(maxTimeout)
						require.NotEqual(t, "HostUID123", testBed.conf.landscapeAgentUID, "Landscape UID should not have been assigned")
						return
					}

					require.Eventually(t, func() bool {
						testBed.conf.mu.Lock()
						defer testBed.conf.mu.Unlock()

						return testBed.conf.landscapeAgentUID == "HostUID123"
					}, maxTimeout, 100*time.Millisecond, "Landscape client should have overridden the initial UID sent by the server")
				})
		})
	}
}

func TestReceiveCommandStartStop(t *testing.T) {
	// The Start and Stop tests are almost identical so they are merged into a single table.

	t.Parallel()

	type command bool

	const (
		start command = false
		stop  command = true
	)

	testCases := map[string]struct {
		dontRegisterDistro bool
		wslErr             bool
		cmd                command

		wantState wsl.State
		wantErr   bool
	}{
		"Success with command Start": {cmd: start, wantState: wsl.Running},
		"Success with command Stop":  {cmd: stop, wantState: wsl.Stopped},

		"Error with Start when the distro does not exist": {cmd: start, dontRegisterDistro: true, wantErr: true},
		"Error with Stop when the distro does not exist":  {cmd: stop, dontRegisterDistro: true, wantErr: true},

		"Error with Start when WSL returns error": {cmd: start, wslErr: true, wantErr: true},
		"Error with Stop when WSL returns error":  {cmd: stop, wslErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.dontRegisterDistro},
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.wslErr {
						testBed.wslMock.WslLaunchInteractiveError = true
					}

					if tc.cmd == start {
						return &landscapeapi.Command{
							Cmd: &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: testBed.distro.Name()}},
						}
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 10 * time.Second
					const tickRate = time.Second

					if tc.wantErr {
						ok, _ := checkEventuallyState(t, testBed.distro, tc.wantState, maxTimeout, tickRate)
						require.False(t, ok, "State %q should never have been reached", tc.wantState)
						return
					}

					ok, state := checkEventuallyState(t, testBed.distro, tc.wantState, maxTimeout, tickRate)
					require.True(t, ok, "Distro never reached %q state. Last state: %q", tc.wantState, state)
				})
		})
	}
}

func TestInstall(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		distroAlredyInstalled bool
		emptyDistroName       bool
		wslInstallErr         bool
		appxDoesNotExist      bool

		wantInstalled   bool
		wantNonRootUser bool
	}{
		"Success": {wantInstalled: true, wantNonRootUser: true},

		"Error when the distroname is empty":         {emptyDistroName: true},
		"Error when the Appx does not exist":         {appxDoesNotExist: true},
		"Error when the distro is already installed": {distroAlredyInstalled: true, wantInstalled: true},
		"Error when the distro fails to install":     {wslInstallErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := distroSettings{
				name: testDistroAppx,
			}

			if tc.appxDoesNotExist {
				// WSLMock Install only accepts ubuntu-22.04
				settings.name = wsltestutils.RandomDistroName(t)
			}

			if tc.distroAlredyInstalled {
				settings.install = true
			}

			testReceiveCommand(t, settings,
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					var distroName string
					if !tc.emptyDistroName {
						distroName = testBed.distro.Name()
					}

					if tc.wslInstallErr {
						testBed.wslMock.InstallError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: distroName}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const timeout = 10 * time.Second // Installation can take a while

					if tc.wantInstalled {
						require.Eventually(t, func() bool {
							registered, err := testBed.distro.IsRegistered()
							if err != nil {
								return false
							}
							return registered
						}, timeout, 100*time.Millisecond, "Distro should have been registered")

						if tc.wantNonRootUser {
							conf, err := testBed.distro.GetConfiguration()
							require.NoError(t, err, "GetConfiguration should return no error")
							require.NotEqual(t, uint32(0), conf.DefaultUID, "Default user should have been changed from root")
						}
						return
					}

					time.Sleep(timeout)

					distroExists, err := testBed.distro.IsRegistered()
					require.NoError(t, err, "IsRegistered should return no error")
					require.False(t, distroExists, "Distro should not have been registered")
				})
		})
	}
}

func TestUninstall(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		distroNotInstalled bool
		wslUninstallErr    bool

		wantNotRegistered bool
	}{
		"Success": {wantNotRegistered: true},

		"Error when the distroname does not match any distro": {distroNotInstalled: true, wantNotRegistered: true},
		"Error when the distro fails to uninstall":            {wslUninstallErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.distroNotInstalled},
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.wslUninstallErr {
						testBed.wslMock.WslUnregisterDistributionError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 20 * time.Second // Uninstalling can take a while

					if tc.wantNotRegistered {
						ok, _ := checkEventuallyState(t, testBed.distro, wsl.NonRegistered, maxTimeout, time.Second)
						require.True(t, ok, "Distro should not be registered")
						return
					}

					time.Sleep(maxTimeout)
					distroExists, err := testBed.distro.IsRegistered()
					require.NoError(t, err, "IsRegistered should return no error")
					require.True(t, distroExists, "Existing distro should still have been unregistered")
				})
		})
	}
}

func TestSetDefaultDistro(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		distroNotInstalled bool
		wslSetDefaultErr   bool
		alreadyDefault     bool

		wantSetAsDefault bool
	}{
		"Success":                             {wantSetAsDefault: true},
		"Success when it was already default": {alreadyDefault: true, wantSetAsDefault: true},

		"Error when the distro name does not match an existing distro": {distroNotInstalled: true},
		"Error when WSL SetDefault fails":                              {wslSetDefaultErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.distroNotInstalled},
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if !tc.alreadyDefault {
						name, _ := wsltestutils.RegisterDistro(t, testBed.ctx, false)
						d := wsl.NewDistro(testBed.ctx, name)
						err := d.SetAsDefault()
						require.NoError(t, err, "Setup: could not set another distro as default")
					}

					if tc.wslSetDefaultErr {
						testBed.wslMock.SetAsDefaultError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 20 * time.Second // Uninstalling can take a while

					if tc.wantSetAsDefault {
						require.Eventually(t, func() bool {
							d, err := wsl.DefaultDistro(testBed.ctx)
							if err != nil {
								return false
							}
							return d.Name() == testBed.distro.Name()
						}, maxTimeout, time.Second, "Distro should have been made default")
					} else {
						time.Sleep(maxTimeout)
						d, err := wsl.DefaultDistro(testBed.ctx)
						require.NoError(t, err, "DefaultDistro should return no error")
						require.NotEqual(t, testBed.distro.Name(), d.Name(), "Distro should not have been default")
					}
				})
		})
	}
}

func TestSetShutdownHost(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		wslShutdownErr bool

		wantStopped bool
	}{
		"Success": {wantStopped: true},

		"Error when the WSL Shutdown fails": {wslShutdownErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: true},
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					d := wsl.NewDistro(testBed.ctx, testBed.distro.Name())
					err := d.Command(testBed.ctx, "exit 0").Run()
					require.NoError(t, err, "Setup: could not start distro")

					if tc.wslShutdownErr {
						testBed.wslMock.ShutdownError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 5 * time.Second

					if !tc.wantStopped {
						time.Sleep(maxTimeout)
						state := wsltestutils.DistroState(t, testBed.ctx, testBed.distro.Name())
						require.Equal(t, "Running", state, "Distro should not have stopped")
						return
					}

					require.Eventually(t, func() bool {
						return wsltestutils.DistroState(t, testBed.ctx, testBed.distro.Name()) == "Stopped"
					}, maxTimeout, time.Second, "Distro should have stopped")
				})
		})
	}
}

// commandTestBed is a bag of data with all the necessary utils to run executor tests.
type commandTestBed struct {
	ctx context.Context

	conf   *mockConfig
	distro *wsl.Distro
	db     *database.DistroDB

	serverService *landscapemockservice.Service
	clientService *landscape.Service

	wslMock *wslmock.Backend
}

// distroSettings tells testReceiveCommand what the test distro should be like.
type distroSettings struct {
	install bool

	// set name to empty to auto-generate one
	name string
}

// testReceiveCommand contains all the boilerplate necessary to test the Landscape executor.
//
// Before testSetup:
//   - Set up the mock WSL
//   - Set up the agent components (config, database...)
//   - Set up the mock Landscape server
//   - Set up the landscape client
//   - Register a distro to test
//
// Then, testSetup is called. After this:
//   - Send the command
//
// Then, testAssertions is called.
func testReceiveCommand(t *testing.T, distrosettings distroSettings, testSetup func(*commandTestBed) *landscapeapi.Command, testAssertions func(*commandTestBed)) {
	t.Helper()
	var tb commandTestBed

	if !wsl.MockAvailable() {
		t.Skip("This test can only run with the mock")
	}

	// Set up WSL mock
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tb.wslMock = wslmock.New()
	ctx = wsl.WithMock(ctx, tb.wslMock)

	tb.ctx = ctx

	// Set up Landscape server
	lis, server, service := setUpLandscapeMock(t, ctx, "localhost:", "")
	context.AfterFunc(ctx, func() { _ = lis.Close() })

	tb.serverService = service

	//nolint: errcheck // We know it is going to exit with "context cancelled"
	go server.Serve(lis)
	context.AfterFunc(ctx, func() { server.Stop() })

	// Set up agent components (config, database, etc.)
	if tb.conf == nil {
		tb.conf = newMockConfig(ctx)
		tb.conf.proToken = "TOKEN"
		tb.conf.landscapeClientConfig = executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr())
	}

	db, err := database.New(ctx, t.TempDir(), tb.conf)
	require.NoError(t, err, "Setup: database New should not return an error")

	tb.db = db

	// Set up test distro
	if distrosettings.name == "" {
		distrosettings.name = wsltestutils.RandomDistroName(t)
	}

	if distrosettings.install {
		d := wsl.NewDistro(ctx, distrosettings.name)
		tb.distro = &d

		err = d.Register(fakeRootFS(t))
		require.NoError(t, err) // Error messsage is explanatory enough

		dbDistro, err := db.GetDistroAndUpdateProperties(ctx, d.Name(), distro.Properties{})
		require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")
		context.AfterFunc(ctx, func() { dbDistro.Cleanup(ctx) })
	} else {
		d := wsl.NewDistro(ctx, distrosettings.name)
		tb.distro = &d
	}

	// Set up Landscape client
	clientService, err := landscape.New(ctx, tb.conf, tb.db, landscape.WithHostname("HOSTNAME"))
	require.NoError(t, err, "Landscape NewClient should not return an error")

	err = clientService.Connect()
	require.NoError(t, err, "Setup: Connect should return no errors")

	tb.clientService = clientService
	context.AfterFunc(ctx, func() { tb.clientService.Stop(ctx) })

	require.Eventually(t, func() bool {
		return clientService.Connected() && tb.conf.landscapeAgentUID != "" && service.IsConnected(tb.conf.landscapeAgentUID)
	}, 10*time.Second, 100*time.Millisecond, "Setup: Landscape server and client never made a connection")

	// Exectute test setup
	command := testSetup(&tb)

	// Send (and receive command)
	err = tb.serverService.SendCommand(ctx, tb.conf.landscapeAgentUID, command)
	require.NoError(t, err, "Setup: SendCommand should return no error")

	// Execute test assertions
	testAssertions(&tb)
}

func fakeRootFS(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	rootfs := filepath.Join(dir, "empty.tar.gz")
	err := os.WriteFile(rootfs, []byte{}, 0600)
	require.NoError(t, err, "Setup: could not write empty fake rootfs")

	return rootfs
}

func checkEventuallyState(t *testing.T, d interface{ State() (wsl.State, error) }, wantState wsl.State, waitFor, tick time.Duration) (ok bool, lastState wsl.State) {
	t.Helper()

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return false, lastState
		case <-ticker.C:
			var err error
			lastState, err = d.State()
			require.NoError(t, err, "disto State should return no error")
			if lastState == wantState {
				return true, lastState
			}
		}
	}
}
