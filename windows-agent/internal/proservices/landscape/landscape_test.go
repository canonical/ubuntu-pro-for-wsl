package landscape_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/landscapemockservice"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"google.golang.org/grpc"
)

func TestConnect(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		precancelContext   bool
		serverNotAvailable bool
		landscapeURLErr    bool
		tokenErr           bool

		wantErr           bool
		wantDistroSkipped bool
	}{
		"Success": {},

		"Error when the context is cancelled before Connected": {precancelContext: true, wantErr: true},
		"Error when the landscape URL cannot be retrieved":     {landscapeURLErr: true, wantErr: true},
		"Error when the server cannot be reached":              {serverNotAvailable: true, wantErr: true},
		"Error when the first-contact SendUpdatedInfo fails ":  {tokenErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0") // Autoselect port
			require.NoError(t, err, "Setup: can't listen")
			defer lis.Close()

			server := grpc.NewServer()
			mockService := landscapemockservice.New()
			landscapeapi.RegisterLandscapeHostAgentServer(server, mockService)

			conf := &mockConfig{
				proToken:     "TOKEN",
				landscapeURL: lis.Addr().String(),

				// We trigger an error on first-contact SendUpdatedInfo by erroring out in conf.ProToken()
				proTokenErr: tc.tokenErr,

				// We trigger an earlier error by erroring out on LandscapeURL
				landscapeURLErr: tc.landscapeURLErr,
			}

			if !tc.serverNotAvailable {
				//nolint:errcheck // We don't care about these errors
				go server.Serve(lis)
				defer server.Stop()
			}

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := testutils.RegisterDistro(t, ctx, true)
			_, err = db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")

			client, err := landscape.NewClient(conf, db)
			require.NoError(t, err, "Setup: NewClient should return no errrors")

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			if tc.precancelContext {
				cancel()
			}

			err = client.Connect(ctx)
			if tc.wantErr {
				require.Error(t, err, "Connect should return an error")
				require.False(t, client.Connected(), "Connected should have returned false after failing to connect")
				return
			}
			require.NoError(t, err, "Connect should return no errors")
			defer client.Disconnect()

			require.True(t, client.Connected(), "Connected should have returned false after succeeding to connect")

			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 0
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

			client.Disconnect()
			require.NotPanics(t, client.Disconnect, "client.Disconnect should not panic, even when called twice")

			require.False(t, client.Connected(), "Connected should have returned false after disconnecting")

			server.Stop()
			lis.Close()

			messages := mockService.MessageLog()
			require.Len(t, messages, 1, "Exactly one message should've been sent to Landscape")
		})
	}
}

func TestSendUpdatedInfo(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		tokenErr bool
		stateErr bool

		precancelContext     bool
		disconnectBeforeSend bool
		distroIsRunning      bool
		distroIsUnregistered bool

		wantErr           bool
		wantDistroSkipped bool
	}{
		"Success with a stopped distro":                     {},
		"Success with a running distro":                     {distroIsRunning: true},
		"Success when the distro State cannot be retreived": {stateErr: true, wantDistroSkipped: true},

		"Error when the token cannot be retreived":                           {tokenErr: true, wantErr: true},
		"Error when attempting to SendUpdatedInfo after having disconnected": {disconnectBeforeSend: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				mock := wslmock.New()
				mock.StateError = tc.stateErr
				ctx = wsl.WithMock(ctx, mock)
			}

			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0") // Autoselect port
			require.NoError(t, err, "Setup: can't listen")
			defer lis.Close()

			server := grpc.NewServer()
			mockService := landscapemockservice.New()
			landscapeapi.RegisterLandscapeHostAgentServer(server, mockService)

			conf := &mockConfig{
				proToken:     "TOKEN",
				landscapeURL: lis.Addr().String(),
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := testutils.RegisterDistro(t, ctx, true)
			props := distro.Properties{
				DistroID:    "Cool Ubuntu",
				VersionID:   "NewerThanYours",
				PrettyName:  "😎 Cool guy 🎸",
				Hostname:    "CoolMachine",
				ProAttached: true,
			}

			distro, err := db.GetDistroAndUpdateProperties(ctx, distroName, props)
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")

			const hostname = "HOSTNAME"

			client, err := landscape.NewClient(conf, db, landscape.WithHostname(hostname))
			require.NoError(t, err, "Landscape NewClient should not return an error")

			if tc.distroIsRunning {
				err := distro.LockAwake()
				//nolint:errcheck // Nothing we can do about it
				defer distro.ReleaseAwake()
				require.NoError(t, err, "Setup: could not keep distro alive")
			} else {
				d := wsl.NewDistro(ctx, distroName)
				err := d.Terminate()
				require.NoError(t, err, "Setup: could not terminate the distro")
			}

			err = client.Connect(ctx)
			require.NoError(t, err, "Setup: Connect should return no errors")
			defer client.Disconnect()

			// Defining wants
			wantID := "THIS_IS_AN_ID"
			wantHostname := hostname
			wantHostToken := conf.proToken
			wantDistroID := distroName
			wantDistroName := props.Hostname
			wantDistroVersionID := props.VersionID
			wantDistroState := landscapeapi.InstanceState_Stopped
			if tc.distroIsRunning {
				wantDistroState = landscapeapi.InstanceState_Running
			}

			// Asserting on the first-contact SendUpdatedInfo
			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 0
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

			messages := mockService.MessageLog()
			require.Len(t, messages, 1, "Exactly one message should've been sent to Landscape")
			msg := &messages[0] // Pointer to avoid copying mutex

			assert.Equal(t, wantID, msg.Id, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")

			if tc.wantDistroSkipped {
				require.Empty(t, msg.Instances, "No distro should've been sent to Landscape")
			} else {
				require.Len(t, msg.Instances, 1, "Exactly one distro should've been sent to Landscape")
				got := msg.Instances[0]
				assert.Equal(t, wantDistroID, got.Id, "Mismatch between local distro Id and that received by the server")
				assert.Equal(t, wantDistroName, got.Name, "Mismatch between local distro Name and that received by the server")
				assert.Equal(t, wantDistroVersionID, got.VersionId, "Mismatch between local distro VersionId and that received by the server")
				assert.Equal(t, wantDistroState, got.InstanceState, "Mismatch between local distro InstanceState and that received by the server")
			}

			// Exiting if previous assert battery failed
			if t.Failed() {
				t.FailNow()
			}

			// Setting up SendUpdatedInfo
			conf.proTokenErr = tc.tokenErr
			conf.proToken = "NEW_TOKEN"

			if tc.disconnectBeforeSend {
				client.Disconnect()
			}

			wantHostToken = conf.proToken

			if !tc.distroIsRunning {
				d := wsl.NewDistro(ctx, distroName)
				err := d.Terminate()
				require.NoError(t, err, "Setup: could not terminate distro")
			}

			err = client.SendUpdatedInfo(ctx)
			if tc.wantErr {
				require.Error(t, err, "SendUpdatedInfo should have returned an error")
				return
			}
			require.NoError(t, err, "SendUpdatedInfo should send no error")

			// Asserting on the second SendUpdatedInfo
			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 1
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a second message from the client")

			messages = mockService.MessageLog()
			require.Len(t, messages, 2, "Exactly two messages should've been sent to Landscape")
			msg = &messages[1] // Pointer to avoid copying mutex

			assert.Equal(t, wantID, msg.Id, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host hostname and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")
			if tc.wantDistroSkipped {
				require.Empty(t, msg.Instances, "No distro should've been sent to Landscape")
			} else {
				require.Len(t, msg.Instances, 1, "Exactly one distro should've been sent to Landscape")
				got := msg.Instances[0]
				assert.Equal(t, wantDistroID, got.Id, "Mismatch between local distro Id and that received by the server")
				assert.Equal(t, wantDistroName, got.Name, "Mismatch between local distro Name and that received by the server")
				assert.Equal(t, wantDistroVersionID, got.VersionId, "Mismatch between local distro VersionId and that received by the server")
				assert.Equal(t, wantDistroState, got.InstanceState, "Mismatch between local distro InstanceState and that received by the server ")
			}
		})
	}
}

type command int

const (
	cmdStart command = iota
	cmdStop
	cmdInstall
	cmdUninstall
	cmdSetDefault
	cmdShutdownHost
)

const (
	testAppx       = "CanonicalGroupLimited.Ubuntu22.04LTS" // The name of the Appx
	testDistroAppx = "Ubuntu-22.04"                         // The name used in `wsl --install <DISTRO>`
)

func TestReceiveCommands(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		command command
		mockErr bool

		wantFailure bool
	}{
		"Success receiving a Start command": {command: cmdStart},
		"Error receiving a Start command":   {command: cmdStart, mockErr: true, wantFailure: true},

		"Success receiving a Stop command": {command: cmdStop},
		"Error receiving a Stop command":   {command: cmdStop, mockErr: true, wantFailure: true},

		"Success receiving a Install command": {command: cmdInstall},
		"Error receiving a Install command":   {command: cmdInstall, mockErr: true, wantFailure: true},

		"Success receiving a Uninstall command": {command: cmdUninstall},
		"Error receiving a Uninstall command":   {command: cmdUninstall, mockErr: true, wantFailure: true},

		"Success receiving a SetDefault command": {command: cmdSetDefault},
		"Error receiving a SetDefault command":   {command: cmdSetDefault, mockErr: true, wantFailure: true},

		"Success receiving a ShutdownHost command": {command: cmdShutdownHost},
		"Error receiving a ShutdownHost command":   {command: cmdShutdownHost, mockErr: true, wantFailure: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			enableMockErrors := func() {}
			if wsl.MockAvailable() {
				t.Parallel()
				mock := wslmock.New()
				ctx = wsl.WithMock(ctx, mock)
				if tc.mockErr {
					enableMockErrors = func() {
						mock.WslLaunchInteractiveError = true      // Breaks start
						mock.InstallError = true                   // Breaks install
						mock.WslUnregisterDistributionError = true // Breaks uninstall
						mock.SetAsDefaultError = true              // Breaks SetDefault
						mock.ShutdownError = true                  // Breaks shutdown
					}
				}
			} else if tc.mockErr {
				t.Skip("This test can only run with the mock")
			}

			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0") // Autoselect port
			require.NoError(t, err, "Setup: can't listen")
			defer lis.Close()

			server := grpc.NewServer()
			service := landscapemockservice.New()
			landscapeapi.RegisterLandscapeHostAgentServer(server, service)

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			conf := &mockConfig{
				proToken:     "TOKEN",
				landscapeURL: lis.Addr().String(),
			}

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			var d *distro.Distro
			if tc.command != cmdInstall {
				distroName, _ := testutils.RegisterDistro(t, ctx, true)
				d, err = db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
				require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")
			}

			command := commandSetup(t, ctx, tc.command, d)
			if tc.command == cmdStop && !tc.mockErr {
				// We need to LockAwake, otherwise ReleaseAwake will error out
				require.NoError(t, d.LockAwake(), "Setup: could not lock distro awake")
			}

			const hostname = "HOSTNAME"
			client, err := landscape.NewClient(conf, db, landscape.WithHostname(hostname))
			require.NoError(t, err, "Landscape NewClient should not return an error")

			err = client.Connect(ctx)
			require.NoError(t, err, "Setup: Connect should return no errors")
			defer client.Disconnect()

			require.Eventually(t, func() bool {
				return service.IsConnected(hostname) && client.Connected()
			}, 10*time.Second, 100*time.Millisecond, "Landscape server and client never made a connection")

			enableMockErrors()

			err = service.SendCommand(ctx, hostname, command)
			require.NoError(t, err, "SendCommand should return no error")

			// Allow some time for the message to be sent, received, and executed.
			time.Sleep(time.Second)

			if wsl.MockAvailable() && tc.command == cmdInstall || tc.command == cmdUninstall {
				// Appx state cannot be mocked
				return
			}

			if tc.command == cmdStop && tc.mockErr {
				// There is no way to assert on this function failing, as it is indistiguishable
				// from succeeding. I can fail two ways:
				//
				// - If Start was not called before. But the effect is the same as in success: the distro will be asleep.
				// - If the distro is no longer valid. Then the command takes no effect so we cannot assert on it.
				//
				// We still have the test case to exercise the code and ensure that it at least does not panic.
				return
			}

			requireCommandResult(t, ctx, tc.command, d, !tc.wantFailure)
		})
	}
}

//nolint:revive // testing.T goes before context
func commandSetup(t *testing.T, ctx context.Context, command command, distro *distro.Distro) *landscapeapi.Command {
	t.Helper()

	var r landscapeapi.Command

	switch command {
	case cmdStart:
		r.Cmd = &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: distro.Name()}}
	case cmdStop:
		r.Cmd = &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: distro.Name()}}
	case cmdInstall:
		r.Cmd = &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: testDistroAppx}}
		t.Cleanup(func() {
			d := wsl.NewDistro(ctx, testDistroAppx)
			_ = d.Uninstall(ctx)
		})
	case cmdUninstall:
		require.NoError(t, wsl.Install(ctx, testDistroAppx), "Setup: could not install Ubuntu-22.04")
		r.Cmd = &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: testDistroAppx}}
		t.Cleanup(func() {
			d := wsl.NewDistro(ctx, testDistroAppx)
			_ = d.Uninstall(ctx)
		})
	case cmdSetDefault:
		otherDistro, _ := testutils.RegisterDistro(t, ctx, false)
		d := wsl.NewDistro(ctx, otherDistro)
		err := d.SetAsDefault()
		require.NoError(t, err, "Setup: could not set another distro as default")
		r.Cmd = &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: distro.Name()}}
	case cmdShutdownHost:
		d := wsl.NewDistro(ctx, distro.Name())
		err := d.Command(ctx, "exit 0").Run()
		require.NoError(t, err, "Setup: could not start distro")
		r.Cmd = &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}}
	default:
		require.FailNowf(t, "Setup", "Unknown command type %d", command)
	}

	return &r
}

// requireCommandResult checks that a certain command has been executed in the machine
// by measuring its effect on the targeted distro. Set wantSuccess to true if you want
// to assert that the command completed successfully, and set it to false to assert it
// did not complete.
//
//nolint:revive // testing.T goes before context
func requireCommandResult(t *testing.T, ctx context.Context, command command, distro *distro.Distro, wantSuccess bool) {
	t.Helper()

	switch command {
	case cmdStart:
		ok, state := checkEventuallyState(t, distro, wsl.Running, 10*time.Second, time.Second)
		if wantSuccess {
			require.True(t, ok, "Distro never reached %q state. Last state: %q", wsl.Running, state)
		} else {
			require.False(t, ok, "Distro unexpectedly reached state %q", wsl.Running)
		}
	case cmdStop:
		// We wait a bit longer than WSL sleep time, because we must account for the Landscape server-client
		// interaction completing asyncronously with the test.
		const waitFor = 15 * time.Second
		ok, state := checkEventuallyState(t, distro, wsl.Stopped, waitFor, time.Second)
		if wantSuccess {
			require.True(t, ok, "Distro never reached %q state. Last state: %q", wsl.Running, state)
		} else {
			require.False(t, ok, "Distro unexpectedly reached state %q", wsl.Stopped)
		}
	case cmdInstall:
		inst := isAppxInstalled(t, testAppx)
		if wantSuccess {
			require.True(t, inst, "Appx should have been installed, but it wasn't")
		} else {
			require.False(t, inst, "Appx should not have been installed, but it was")
		}
	case cmdUninstall:
		inst := isAppxInstalled(t, testAppx)
		if wantSuccess {
			require.False(t, inst, "Appx should no longer be installed, but it is")
		} else {
			require.True(t, inst, "Appx should still be installed, but it isn't")
		}
	case cmdSetDefault:
		def, err := wsl.DefaultDistro(ctx)
		require.NoError(t, err, "could not call DefaultDistro")
		if wantSuccess {
			require.Equal(t, distro.Name(), def.Name(), "Test distro should be the default one")
		} else {
			require.NotEqual(t, distro.Name(), def.Name(), "Test distro should not be the default one")
		}
	case cmdShutdownHost:
		gotState, err := distro.State()
		require.NoError(t, err, "Could not read distro state")
		if wantSuccess {
			require.Equal(t, wsl.Stopped, gotState, "Unexpected disto state. Want: %q. Got: %q", wsl.Stopped, gotState)
		} else {
			require.Equal(t, wsl.Running, gotState, "Unexpected disto state. Want: %q. Got: %q", wsl.Running, gotState)
		}
	default:
		require.FailNowf(t, "Setup", "Unknown command type %d", command)
	}
}

func checkEventuallyState(t *testing.T, d *distro.Distro, wantState wsl.State, waitFor, tick time.Duration) (ok bool, lastState wsl.State) {
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

func isAppxInstalled(t *testing.T, appxPackage string) bool {
	t.Helper()
	require.False(t, wsl.MockAvailable(), "This assertion is only valid without the WSL mock")

	cmd := fmt.Sprintf("(Get-AppxPackage -Name %q).Status", appxPackage)
	//nolint:gosec // Command with variable is acceptable in test code
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NoLogo", "-NonInteractive", "-Command", cmd).Output()
	require.NoError(t, err, "Get-AppxPackage should return no error. Stdout: %s", string(out))

	return strings.Contains(string(out), "Ok")
}

type mockConfig struct {
	proToken     string
	landscapeURL string

	proTokenErr     bool
	landscapeURLErr bool
}

func (m mockConfig) ProvisioningTasks(ctx context.Context) ([]task.Task, error) {
	return nil, nil
}

func (m mockConfig) ProToken(ctx context.Context) (string, error) {
	if m.proTokenErr {
		return "", errors.New("Mock error")
	}
	return m.proToken, nil
}

func (m mockConfig) LandscapeURL(ctx context.Context) (string, error) {
	if m.landscapeURLErr {
		return "", errors.New("Mock error")
	}
	return m.landscapeURL, nil
}

func (m mockConfig) Pseudonym() string {
	return "PSEUDONYM"
}

func (m mockConfig) Hostname() string {
	return "HOSTNAME"
}
