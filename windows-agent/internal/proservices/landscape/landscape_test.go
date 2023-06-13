package landscape_test

import (
	"context"
	"errors"
	"net"
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
				err := distro.PushAwake()
				//nolint:errcheck // Nothing we can do about it
				defer distro.PopAwake()
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

func TestReceiveCommands(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		command command
	}{
		"Success receiving a Start command":        {command: cmdStart},
		"Success receiving a Stop command":         {command: cmdStop},
		"Success receiving a Install command":      {command: cmdInstall},
		"Success receiving a Uninstall command":    {command: cmdUninstall},
		"Success receiving a SetDefault command":   {command: cmdSetDefault},
		"Success receiving a ShutdownHost command": {command: cmdShutdownHost},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			distroName, _ := testutils.RegisterDistro(t, ctx, true)
			command := commandSetup(t, ctx, tc.command, distroName)

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

			distro, err := db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")

			const hostname = "HOSTNAME"
			client, err := landscape.NewClient(conf, db, landscape.WithHostname(hostname))
			require.NoError(t, err, "Landscape NewClient should not return an error")

			err = client.Connect(ctx)
			require.NoError(t, err, "Setup: Connect should return no errors")
			defer client.Disconnect()

			require.Eventually(t, func() bool {
				return service.IsConnected(hostname) && client.Connected()
			}, 10*time.Second, 100*time.Millisecond, "Landscape server and client never made a connection")

			err = service.SendCommand(ctx, hostname, command)
			require.NoError(t, err, "SendCommand should return no error")

			// Allow some time for the message to be sent, received, and executed.
			time.Sleep(time.Second)

			requireCommandResult(t, ctx, tc.command, distro)
		})
	}
}

//nolint:revive // testing.T goes before context
func commandSetup(t *testing.T, ctx context.Context, command command, distroName string) *landscapeapi.Command {
	t.Helper()

	var r landscapeapi.Command

	switch command {
	case cmdStart:
		t.Skip("Skipping because it is not implemented")
		// r.Cmd = &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: id}}
	case cmdStop:
		t.Skip("Skipping because it is not implemented")
		// r.Cmd = &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: id}}
	case cmdInstall:
		t.Skip("Skipping because it is not implemented")
		// r.Cmd = &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: id}}
	case cmdUninstall:
		t.Skip("Skipping because it is not implemented")
		// r.Cmd = &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: id}}
	case cmdSetDefault:
		otherDistro, _ := testutils.RegisterDistro(t, ctx, false)
		d := wsl.NewDistro(ctx, otherDistro)
		err := d.SetAsDefault()
		require.NoError(t, err, "Setup: could not set another distro as default")
		r.Cmd = &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: distroName}}
	case cmdShutdownHost:
		d := wsl.NewDistro(ctx, distroName)
		err := d.Command(ctx, "exit 0").Run()
		require.NoError(t, err, "Setup: could not start distro")
		r.Cmd = &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}}
	default:
		require.FailNowf(t, "Setup", "Unknown command type %d", command)
	}

	return &r
}

// requireCommandResult asserts that a certain command has been executed in the machine
// by measuring its effect on the targeted distro.
//
//nolint:revive // testing.T goes before context
func requireCommandResult(t *testing.T, ctx context.Context, command command, distro *distro.Distro) {
	t.Helper()

	switch command {
	case cmdStart:
		panic("this test should have been skipped")
	case cmdStop:
		panic("this test should have been skipped")
	case cmdInstall:
		panic("this test should have been skipped")
	case cmdUninstall:
		panic("this test should have been skipped")
	case cmdSetDefault:
		def, err := wsl.DefaultDistro(ctx)
		require.NoError(t, err, "could not call DefaultDistro")
		require.Equal(t, distro.Name(), def.Name(), "Distro was not set as default")
	case cmdShutdownHost:
		s, err := distro.State()
		require.NoError(t, err, "Could not read distro state")
		require.Equalf(t, wsl.Stopped, s, "distro should have been stopped (want: %s, got: %s)", wsl.Stopped.String(), s.String())
	default:
		require.FailNowf(t, "Setup", "Unknown command type %d", command)
	}
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
