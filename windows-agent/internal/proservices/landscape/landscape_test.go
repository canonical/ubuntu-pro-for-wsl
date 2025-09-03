package landscape_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/landscape/landscapemockservice"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/cloudinit"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

const defaultLandscapeConfig = `
[host]
url = "{{ .HostURL }}"

[client]
account_name = testuser
registration_key = password1
log_level = info
`

func TestNew(t *testing.T) {
	testCases := map[string]struct {
		breakHome bool

		wantError bool
	}{
		"Success": {},

		"Error when no user home is available": {breakHome: true, wantError: true},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		conf := &mockConfig{}
		db, err := database.New(ctx, t.TempDir())
		require.NoError(t, err, "Setup: database New should not return an error")

		// Note that these tests cannot be run in parallel due to manipulating
		// these test environment variables
		if tc.breakHome {
			t.Setenv("UserProfile", "")
			t.Setenv("HOME", "")
		}

		var cloudInit mockCloudInit
		inst, err := landscape.New(ctx, conf, db, &cloudInit, nil)

		if tc.wantError {
			require.Error(t, err, "Creating a new Landscape instance should fail")
			return
		}
		require.NoError(t, err, "Creating a new Landscape instance should not fail")
		require.NotNil(t, inst, "Landscape instance should not be nil")
	}
}

func TestConnect(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	certPath := t.TempDir()

	testutils.GenerateTempCertificate(t, certPath)

	err := os.WriteFile(filepath.Join(certPath, "bad-certificate.pem"), []byte("This is not a valid certificate."), 0600)
	require.NoError(t, err, "Setup: could not create bad certificate")

	testCases := map[string]struct {
		precancelContext   bool
		serverNotAvailable bool
		serverErrorCode    codes.Code // 0. codes.OK by default.

		landscapeUIDReadErr  bool
		landscapeUIDWriteErr bool

		emptyToken bool
		tokenErr   bool

		clientUsesTLS              bool
		serverUsesTLS              bool
		breakLandscapeClientConfig bool

		breakUIDFile bool
		uid          string

		wantErr           bool
		wantNotConnected  bool
		wantDistroSkipped bool
		wantSingleMessage bool
	}{
		"Success":                         {},
		"Success in non-first contact":    {uid: "123", wantSingleMessage: true},
		"Success with an SSL certificate": {clientUsesTLS: true, serverUsesTLS: true},

		// These tests are for the error cases when the error is logged but not returned
		"Silent error when the config is empty":                   {wantNotConnected: true},
		"Silent error when the landscape URL is missing":          {wantNotConnected: true},
		"Silent error when the landscape host section is missing": {wantNotConnected: true},
		"Silent error when the Ubuntu Pro token is missing":       {emptyToken: true, wantNotConnected: true},

		"Error when the context is cancelled before Connected": {precancelContext: true, wantErr: true},
		"Error when the landscape UID cannot be retrieved":     {landscapeUIDReadErr: true, wantErr: true},
		"Error when the landscape UID cannot be stored":        {landscapeUIDWriteErr: true, wantErr: true},
		"Error when the server cannot be reached":              {serverNotAvailable: true, wantErr: true},
		"Error when the server returns permission denied":      {serverErrorCode: codes.PermissionDenied, wantErr: true},
		"Error when the server returns invalid argument":       {serverErrorCode: codes.InvalidArgument, wantErr: true},
		"Error when the first-contact SendUpdatedInfo fails":   {tokenErr: true, wantErr: true},
		"Error when the config cannot be accessed":             {breakLandscapeClientConfig: true, wantErr: true},
		"Error when the config cannot be parsed":               {wantErr: true},
		"Error when the SSL certificate cannot be read":        {clientUsesTLS: true, serverUsesTLS: true, wantErr: true},
		"Error when the SSL certificate is not valid":          {clientUsesTLS: true, serverUsesTLS: true, wantErr: true},
		"Error when the SSL certificate is not trusted":        {clientUsesTLS: true, serverUsesTLS: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if !tc.clientUsesTLS {
				ctx = context.WithValue(ctx, landscape.InsecureCredentials, true)
			}

			p := ""
			if tc.serverUsesTLS {
				p = certPath
			}

			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			var connErr error
			if tc.serverErrorCode != codes.OK {
				connErr = status.Error(tc.serverErrorCode, "Mock error")
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", p, connErr)
			defer lis.Close()

			conf := &mockConfig{
				proToken: "TOKEN",
			}

			// We trigger an error on first-contact SendUpdatedInfo by erroring out in conf.ProToken()
			conf.proTokenErr = tc.tokenErr

			// We trigger errors trying to read or write to/from the registry
			conf.landscapeUIDErr = tc.landscapeUIDReadErr
			conf.setLandscapeUIDErr = tc.landscapeUIDWriteErr

			// We trigger an error when deciding to use a certificate or not
			conf.landscapeConfigErr = tc.breakLandscapeClientConfig

			conf.landscapeAgentUID = tc.uid

			if tc.emptyToken {
				conf.proToken = ""
			}

			lconf := defaultLandscapeConfig
			if fixture, err := os.ReadFile(filepath.Join(testutils.TestFixturePath(t), "landscape.conf")); err != nil {
				require.ErrorIs(t, err, os.ErrNotExist, "Setup: could not load landscape config")
				// Fixture does not exist: use base Landscape confing
			} else {
				// Fixture exists: override the Landscape config
				lconf = string(fixture)
			}

			conf.landscapeClientConfig = executeLandscapeConfigTemplate(t, lconf, certPath, lis.Addr())

			if !tc.serverNotAvailable {
				//nolint:errcheck // We don't care about these errors
				go server.Serve(lis)
				defer server.Stop()
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
			_, err = db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")

			var cloudInit mockCloudInit
			service, err := landscape.New(ctx, conf, db, &cloudInit, nil, landscape.WithHomeDir(t.TempDir()))
			require.NoError(t, err, "Setup: NewClient should return no errors")

			if tc.precancelContext {
				cancel()
			}

			err = service.Connect()
			if tc.wantErr {
				require.Error(t, err, "Connect should return an error")
				require.False(t, service.Connected(), "Connected should have returned false after failing to connect")
				return
			}
			require.NoError(t, err, "Connect should return no errors")
			defer service.Stop(ctx)

			if tc.wantNotConnected {
				// Service is disabled due to missing configuration
				require.False(t, service.Connected(), "Connected should have returned false")
				time.Sleep(10 * time.Second)

				err = service.Controller().SendUpdatedInfo(ctx)
				require.NoError(t, err, "SendUpdatedInfo should fail silently when the service is disabled")
				time.Sleep(10 * time.Second)
				require.False(t, service.Connected(), "SendUpdatedInfo should not reconnect when the service is disabled")

				ok := service.Controller().Reconnect(ctx)
				require.False(t, ok, "Reconnect should not succeed when the service is disabled")
				time.Sleep(10 * time.Second)
				require.False(t, service.Connected(), "SendUpdatedInfo should not reconnect when the service is disabled")

				return
			}
			require.True(t, service.Connected(), "Connected should have returned false after succeeding to connect")

			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 0
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

			const timeout = 10 * time.Second
			if tc.wantSingleMessage {
				time.Sleep(timeout)
				require.Len(t, mockService.MessageLog(), 1, "Exactly one message should've been sent to Landscape")
			} else {
				require.Eventually(t, func() bool {
					return len(mockService.MessageLog()) == 2
				}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a second message from the client in response to the UID assignment")
			}

			err = service.Connect()
			require.Error(t, err, "Connect should return an error when already connected")

			service.Stop(ctx)
			require.NotPanics(t, func() { service.Stop(ctx) }, "Stop should not panic, even when called twice")

			require.False(t, service.Connected(), "Connected should have returned false after disconnecting")

			wantUID := tc.uid
			if tc.uid == "" {
				wantUID = "ServerAssignedUID"
			}
			requireHasPrefix(t, wantUID, conf.landscapeAgentUID, "Landscape client UID was not set properly")

			server.Stop()
			lis.Close()
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

		breakWSLRegistry   bool // Needs dontRegisterDistro to be true
		dontRegisterDistro bool

		precancelContext     bool
		disconnectBeforeSend bool
		distroIsRunning      bool
		distroIsUnregistered bool

		wantErr           bool
		wantDistroSkipped bool
	}{
		"Success with a stopped distro":                       {},
		"Success with a running distro":                       {distroIsRunning: true},
		"Success when the distro State cannot be retreived":   {stateErr: true, wantDistroSkipped: true},
		"Success when the default distro cannot be retreived": {breakWSLRegistry: true, dontRegisterDistro: true, wantDistroSkipped: true},

		"Error when the token cannot be retreived":                           {tokenErr: true, wantErr: true},
		"Error when attempting to SendUpdatedInfo after having disconnected": {disconnectBeforeSend: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), landscape.InsecureCredentials, true)
			if wsl.MockAvailable() {
				t.Parallel()
				mock := wslmock.New()
				mock.StateError = tc.stateErr
				mock.OpenLxssKeyError = tc.breakWSLRegistry
				ctx = wsl.WithMock(ctx, mock)
			} else if tc.stateErr {
				t.Skip("This test is skipped because it necessitates the GoWSL mock")
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", "", nil)

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr()),
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database New should not return an error")

			var d *distro.Distro
			var distroName string
			var props distro.Properties

			if !tc.dontRegisterDistro {
				distroName, _ = wsltestutils.RegisterDistro(t, ctx, true)
				props = distro.Properties{
					DistroID:    "Cool Ubuntu",
					VersionID:   "NewerThanYours",
					PrettyName:  "😎 Cool guy 🎸",
					Hostname:    "CoolMachine",
					ProAttached: true,
				}

				d, err = db.GetDistroAndUpdateProperties(ctx, distroName, props)
				require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")
			}

			const hostname = "HOSTNAME"

			var cloudInit mockCloudInit
			service, err := landscape.New(ctx, conf, db, &cloudInit, nil, landscape.WithHostname(hostname), landscape.WithHomeDir(t.TempDir()))
			require.NoError(t, err, "Landscape NewClient should not return an error")

			ctl := service.Controller()

			if !tc.dontRegisterDistro {
				if tc.distroIsRunning {
					err := d.LockAwake()
					//nolint:errcheck // Nothing we can do about it
					defer d.ReleaseAwake()
					require.NoError(t, err, "Setup: could not keep distro alive")
				} else {
					d := wsl.NewDistro(ctx, distroName)
					err := d.Terminate()
					require.NoError(t, err, "Setup: could not terminate the distro")
				}
			}

			err = service.Connect()
			require.NoError(t, err, "Setup: Connect should return no errors")
			defer service.Stop(ctx)

			// Defining wants
			wantUIDprefix := "ServerAssignedUID"
			wantHostname := hostname
			wantHostToken := conf.proToken
			wantAccountName := "testuser"
			wantRegistrationKey := "password1"
			wantDistroID := distroName
			wantDistroName := props.Hostname
			wantDistroVersionID := props.VersionID
			wantDistroState := landscapeapi.InstanceState_Stopped
			if tc.distroIsRunning {
				wantDistroState = landscapeapi.InstanceState_Running
			}

			// Asserting on the first-contact SendUpdatedInfo
			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 1
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

			messages := mockService.MessageLog()
			// Two messages:
			// 1. First-contact message
			// 2. Reply after the assignHost message is completed
			require.Len(t, messages, 2, "Exactly two messages should've been sent to Landscape")

			assert.Empty(t, messages[0].UID, "First UID received by the server should be empty")
			assertHasPrefix(t, wantUIDprefix, messages[1].UID, "Second UID received by the server should be the one assigned by the server")

			msg := &messages[1] // Pointer to avoid copying mutex
			assert.Equal(t, wantAccountName, msg.AccountName, "Mismatch between local account name and that received by the server")
			assert.Equal(t, wantRegistrationKey, msg.RegistrationKey, "Mismatch between local registration key and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")
			require.Equal(t, wantDistroID, msg.DefaultInstanceID, "The only distro that is registered should have been labeled default")

			if tc.wantDistroSkipped {
				require.Empty(t, msg.Instances, "No distro should've been sent to Landscape")
			} else {
				require.Len(t, msg.Instances, 1, "Exactly one distro should've been sent to Landscape")

				// Assigning the Landscape UID can affect the state of the distros, so we assert on the
				// sent received before this assignment
				got := messages[0].Instances[0]
				assert.Equal(t, wantDistroID, got.ID, "Mismatch between local distro Id and that received by the server")
				assert.Equal(t, wantDistroName, got.Name, "Mismatch between local distro Name and that received by the server")
				assert.Equal(t, wantDistroVersionID, got.VersionID, "Mismatch between local distro VersionId and that received by the server")
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
				service.Stop(ctx)
			}

			wantHostToken = conf.proToken

			if !tc.dontRegisterDistro && !tc.distroIsRunning {
				d := wsl.NewDistro(ctx, distroName)
				err := d.Terminate()
				require.NoError(t, err, "Setup: could not terminate distro")
			}

			err = ctl.SendUpdatedInfo(ctx)
			if tc.wantErr {
				require.Error(t, err, "SendUpdatedInfo should have returned an error")
				return
			}
			require.NoError(t, err, "SendUpdatedInfo should send no error")

			// Asserting on the second SendUpdatedInfo
			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 2
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a second message from the client")

			messages = mockService.MessageLog()
			require.Len(t, messages, 3, "Exactly two messages should've been sent to Landscape")
			msg = &messages[2] // Pointer to avoid copying mutex

			assertHasPrefix(t, wantUIDprefix, msg.UID, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantAccountName, msg.AccountName, "Mismatch between local account name and that received by the server")
			assert.Equal(t, wantRegistrationKey, msg.RegistrationKey, "Mismatch between local registration key and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host hostname and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")
			require.Equal(t, wantDistroID, msg.DefaultInstanceID, "The only distro that is registered should have been labeled default")

			if tc.wantDistroSkipped {
				require.Empty(t, msg.Instances, "No distro should've been sent to Landscape")
			} else {
				require.Len(t, msg.Instances, 1, "Exactly one distro should've been sent to Landscape")
				got := msg.Instances[0]
				assert.Equal(t, wantDistroID, got.ID, "Mismatch between local distro Id and that received by the server")
				assert.Equal(t, wantDistroName, got.Name, "Mismatch between local distro Name and that received by the server")
				assert.Equal(t, wantDistroVersionID, got.VersionID, "Mismatch between local distro VersionId and that received by the server")
				assert.Equal(t, wantDistroState, got.InstanceState, "Mismatch between local distro InstanceState and that received by the server ")
			}
		})
	}
}

func requireHasPrefix(t *testing.T, wantPrefix, got string, msgAndArgs ...interface{}) {
	t.Helper()

	if assertHasPrefix(t, wantPrefix, got, msgAndArgs...) {
		return
	}

	t.FailNow()
}

func assertHasPrefix(t *testing.T, wantPrefix, got string, msgAndArgs ...interface{}) bool {
	t.Helper()

	if strings.HasPrefix(got, wantPrefix) {
		return true
	}

	errMsg := fmt.Sprintf("String does not have prefix.\n    Prefix: %s\n    String: %s\n", wantPrefix, got)
	assert.Fail(t, errMsg, msgAndArgs)
	return false
}

func TestAutoReconnection(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		stopEarly bool

		wantErr bool
	}{
		"Success": {},

		"Error when the reconnect petition is cancelled via stopping the client": {stopEarly: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), landscape.InsecureCredentials, true))
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				mock := wslmock.New()
				ctx = wsl.WithMock(ctx, mock)
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", "", nil)
			defer lis.Close()
			defer server.Stop()

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr()),
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database New should not return an error")

			const hostname = "HOSTNAME"

			var cloudInit mockCloudInit
			service, err := landscape.New(ctx, conf, db, &cloudInit, nil, landscape.WithHostname(hostname), landscape.WithHomeDir(t.TempDir()))
			require.NoError(t, err, "Landscape NewClient should not return an error")
			defer service.Stop(ctx)

			err = service.Connect()
			require.Error(t, err, "Connect should have failed because the server is not running")
			require.False(t, service.Connected(), "Client should not be connected because the server is not running")

			ch := make(chan error)
			go func() {
				// This should block until either:
				// - Success: The message has been sent
				// - Error: The context is cancelled or Landscape is stopped
				//
				// We cannot assert on the error here: failed asserts outside the main goroutine cause panics
				ch <- service.Controller().SendUpdatedInfo(ctx)
				close(ch)
			}()

			select {
			case <-ch:
				require.Fail(t, "SendUpdatedInfo should not have returned because there is no connection")
			case <-time.After(20 * time.Second):
			}

			if tc.stopEarly {
				service.Stop(ctx)
				time.Sleep(time.Second) // Allow it to propagate
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			select {
			case <-time.After(20 * time.Second):
				require.Fail(t, "SendUpdatedInfo should have returned")
			case err = <-ch:
			}

			if tc.wantErr {
				require.Error(t, err, "SendUpdatedInfo should have returned an error")
				return
			}
			require.NoError(t, err, "SendUpdatedInfo should have returned no error")

			require.Eventually(t, func() bool {
				return service.Connected()
			}, 5*time.Second, 500*time.Millisecond, "Client should have reconnected after starting the server")

			hosts := mockService.Hosts()
			require.Len(t, hosts, 1, "Only one client should have connected to the Landscape server")
			uid := maps.Keys(hosts)[0]

			ok := monitorDisconnection(t, mockService, uid, func() error {
				return mockService.Disconnect(uid)
			})
			require.True(t, ok, "Client should have disconnected after terminating the connection from the server")

			// Detecting reconnection
			require.Eventually(t, func() bool {
				return mockService.IsConnected(uid)
			}, 10*time.Second, 100*time.Millisecond, "Client should have reconnected after the stream is dropped")

			ok = monitorDisconnection(t, mockService, uid, func() error {
				server.Stop()
				return nil
			})
			require.True(t, ok, "Client should have disconnected after stopping the server")

			// Restart server at the same address
			lis, server, _ = setUpLandscapeMock(t, ctx, lis.Addr().String(), "", nil)
			defer lis.Close()

			//nolint:errcheck // We don't care
			go server.Serve(lis)
			defer server.Stop()

			require.Eventually(t, func() bool {
				return service.Connected()
			}, 60*time.Second, 500*time.Millisecond, "Client should have reconnected after restarting the server")
			// Seems a bit long of a timeout, but the wait-time doubles after each failed attempt,
			// so after 6 failed attempts, we're waiting for 64 seconds.
			//
			// In local testing I have not seen it go beyond 16 seconds, but I'd rather avoid flaky tests.
		})
	}
}

func monitorDisconnection(t *testing.T, landscapeService *landscapemockservice.Service, uid string, trigger func() error) bool {
	t.Helper()

	require.True(t, landscapeService.IsConnected(uid), "Client should be connected before disconnection")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We must already be waiting when we trigger the disconnection. Otherwise, the disconnection may be missed.
	wait := make(chan struct{})
	go func() {
		// Signal that we the client has disconnected
		defer close(wait)

		disconnected := landscapeService.WaitDisconnection(uid)

		// Signal that we're entering the select statement
		wait <- struct{}{}

		select {
		case <-disconnected:
		case <-ctx.Done():
		}
	}()

	// This is as sure as we can get that the goroutine has reached the select statement.
	<-wait
	time.Sleep(time.Second)

	require.NoErrorf(t, trigger(), "Failed to trigger disconnection")

	// Wait for disconnection
	select {
	case <-wait:
		return true
	case <-time.After(30 * time.Second):
		return false
	}
}

func TestReconnect(t *testing.T) {
	t.Parallel()

	requestReconnect := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		s.Controller().Reconnect(ctx)
	}

	changeAddress := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		// We change the address to an equivalent one, so that the reconnect is triggered and the connection succeeds
		c.mu.Lock()
		c.landscapeClientConfig = strings.ReplaceAll(c.landscapeClientConfig, "127.0.0.1", "localhost")
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	changeCertificate := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		// We change the path to an equivalent one, so that the reconnect is triggered and the connection still succeeds
		const sep = filepath.Separator
		from := fmt.Sprintf("%c", sep)       // from: /
		to := fmt.Sprintf("%c.%c", sep, sep) // to:   /./

		c.mu.Lock()
		c.landscapeClientConfig = strings.Replace(c.landscapeClientConfig, from, to, 1)
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	changeProToken := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.proToken = ""
		c.mu.Unlock()

		s.NotifyUbuntuProUpdate(ctx, c.proToken)
	}

	changeRegistrationKey := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.landscapeClientConfig = strings.Replace(c.landscapeClientConfig, "registration_key = password1", "registration_key = password2", 1)
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	changeLogLevel := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.landscapeClientConfig = strings.Replace(c.landscapeClientConfig, "log_level = info", "log_level = debug", 1)
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	changeIrrelevant := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.landscapeClientConfig = c.landscapeClientConfig + "\n[exta]\ninfo=this section does not matter"
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	changeRemoveConfig := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.landscapeClientConfig = ""
		c.mu.Unlock()

		s.NotifyConfigUpdate(ctx, c.landscapeClientConfig, c.landscapeAgentUID)
	}

	testCases := map[string]struct {
		useCertificate bool
		trigger        func(context.Context, *landscape.Service, *mockConfig)

		wantNoDisconnect       bool
		wantNoReconnect        bool
		wantImmediateReconnect bool
	}{
		"Reconnect when explicitly requesting a reconnection": {trigger: requestReconnect, wantImmediateReconnect: true},
		"Reconnect when changing the URL":                     {trigger: changeAddress},
		"Reconnect when changing the certificate path":        {trigger: changeCertificate, useCertificate: true},
		"Reconnect when changing the registration key":        {trigger: changeRegistrationKey},

		"Don't disconnect when changing irrelevant config": {trigger: changeIrrelevant, wantNoDisconnect: true},
		"Don't disconnect when changing client log level":  {trigger: changeLogLevel, wantNoDisconnect: true},

		"Don't reconnect when removing the Ubuntu Pro token": {trigger: changeProToken, wantNoReconnect: true},
		"Don't reconnect when missing configuration":         {trigger: changeRemoveConfig, wantNoReconnect: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			var certPath string
			lcapeConfig := defaultLandscapeConfig
			if tc.useCertificate {
				certPath = t.TempDir()
				testutils.GenerateTempCertificate(t, certPath)
				lcapeConfig = fmt.Sprintf("%s\nssl_public_key = {{ .CertPath }}/cert.pem", defaultLandscapeConfig)
			} else {
				ctx = context.WithValue(ctx, landscape.InsecureCredentials, true)
			}

			lis, server, mockServerService := setUpLandscapeMock(t, ctx, "localhost:", certPath, nil)

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, lcapeConfig, certPath, lis.Addr()),
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database New should not return an error")

			var cloudInit mockCloudInit
			service, err := landscape.New(ctx, conf, db, &cloudInit, nil, landscape.WithHomeDir(t.TempDir()))
			require.NoError(t, err, "Setup: New should not return an error")

			err = service.Connect()
			require.NoError(t, err, "Setup: Connect should not return an error")

			require.Eventually(t, func() bool {
				return service.Connected()
			}, 5*time.Second, 500*time.Millisecond, "Client should have reconnected after restarting the server")

			hosts := mockServerService.Hosts()
			require.Len(t, hosts, 1, "Only one client should have connected to the Landscape server")
			uid := maps.Keys(hosts)[0]

			ok := monitorDisconnection(t, mockServerService, uid, func() error {
				tc.trigger(ctx, service, conf)
				return nil
			})

			if tc.wantNoDisconnect {
				require.False(t, ok, "Client should not have disconnected")
				return
			}
			require.True(t, ok, "Client should have disconnected")

			if tc.wantImmediateReconnect {
				require.True(t, service.Connected(), "Client should have connected before returning")
				return
			}

			maxTimeout := 20 * time.Second
			if tc.wantNoReconnect {
				time.Sleep(maxTimeout)
				require.False(t, service.Connected(), "Client should not have reconnected")
				return
			}

			require.Eventually(t, service.Connected,
				maxTimeout, time.Second, "Client should have connected after reconnection")
		})
	}
}

func TestNotifyConfigUpdate(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testcases := map[string]struct {
		emptyDB   bool
		conf, uid string

		want        string
		wantNoTasks bool
	}{
		"Task contains client conf when UID is not empty":                                       {},
		"Task contains empty client conf":                                                       {conf: "-"},
		"Task contains empty client conf when UID is empty despite submitted conf is not empty": {uid: "-"},
		"Task contains empty client conf when both are empty":                                   {conf: "-", uid: "-"},
		"Task doesn't contain [host] section":                                                   {conf: "[host]\nurl=localhost\n[client]\nurl=another\n", want: "[client]\nurl=another\n"},

		"Tasks are skipped when database is empty":                       {emptyDB: true, wantNoTasks: true},
		"Tasks are skipped when config is invalid INI syntax":            {conf: "INVALID INI SYNTAX", wantNoTasks: true},
		"Tasks are skipped when config contains only the [host] section": {conf: "[host]\nurl=localhost", wantNoTasks: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			switch tc.conf {
			case "":
				tc.conf = "[client]\nhello=world"
				tc.want = tc.conf
			case "-":
				tc.conf = ""
				tc.want = tc.conf
			default:
			}

			switch tc.uid {
			case "":
				tc.uid = "ServerAssignedUID"
			case "-":
				tc.uid = ""
			default:
			}

			storageDir := t.TempDir()
			db, err := database.New(ctx, storageDir)
			require.NoError(t, err, "Setup: database New should not return an error")

			if !tc.emptyDB {
				distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
				d, err := db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
				require.NoError(t, err, "Setup: distro %s GetDistroAndUpdateProperties should return no errors", distroName)
				// Prevents the distro's worker to dequeue tasks, otherwise we race, as we attempt to read pending tasks.
				d.Cleanup(ctx)
			}

			var cloudInit mockCloudInit
			service, err := landscape.New(ctx, &mockConfig{}, db, &cloudInit, nil, landscape.WithHomeDir(t.TempDir()))
			require.NoError(t, err, "Setup: New should not return an error")

			service.NotifyConfigUpdate(ctx, tc.conf, tc.uid)

			// There is no direct way to observe the result of that function other than relying on the implementation details of the task database.
			tasksFiles, err := filepath.Glob(filepath.Join(storageDir, "*.tasks"))
			require.NoError(t, err, "NotifyConfigUpdate: could not list the tasks files storage dir: %s", storageDir)

			if tc.wantNoTasks {
				require.Empty(t, tasksFiles, "NotifyConfigUpdate: should not have created a tasks file when the database is empty")
				return
			}

			b, err := os.ReadFile(tasksFiles[0])
			require.NoError(t, err, "NotifyConfigUpdate: should have caused creation of a tasks file")
			task := strings.TrimSpace(strings.ReplaceAll(string(b), " ", ""))
			require.NotEmpty(t, task, "NotifyConfigUpdate: tasks file should not be empty")
			if tc.uid == "" && tc.conf != "" {
				require.NotContains(t, task, tc.want, "NotifyConfigUpdate: tasks file should not contain the Landscape client config submitted")
			} else {
				require.Contains(t, task, tc.want, "NotifyConfigUpdate: tasks file should contain the Landscape client config submitted")
			}
			require.Contains(t, task, tasks.LandscapeConfigure{}.String(), "NotifyConfigUpdate: tasks file should contain a LandscapeConfigure task")
		})
	}
}

func TestNotifyConfigUpdateWithAgentYaml(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testcases := map[string]struct {
		conf string
	}{

		"Task and agent.yaml don't contain [host] section":    {conf: "[host]\nurl=localhost:1234\n[client]\ncomputer_title=another\n"},
		"Task and agent.yaml only contains [client] section)": {conf: "[irrelevant]\nnothing=important\n[host]\nurl=localhost:1234\n[client]\ncomputer_title=another\n"},
		"Task and agent.yaml with default tags":               {conf: "[host]\nurl=localhost:1234\n[client]\ncomputer_title=another\n"},
		"Task and agent.yaml with supplied tags":              {conf: "[host]\nurl=localhost:1234\n[client]\ntags=another\n"},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			storageDir := t.TempDir()
			db, err := database.New(ctx, storageDir)
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
			d, err := db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: distro %s GetDistroAndUpdateProperties should return no errors", distroName)
			// Let's stop the distro right away so it's worker don't race with us for the tasks file.
			d.Cleanup(ctx)

			homedir := t.TempDir()
			c := config.New(ctx, storageDir)
			cloudInit, err := cloudinit.New(ctx, c, homedir)
			require.NoError(t, err, "Setup: cloudinit New should not return an error")
			service, err := landscape.New(ctx, c, db, &cloudInit, nil, landscape.WithHomeDir(homedir))
			require.NoError(t, err, "Setup: New should not return an error")

			c.SetLandscapeNotifier(func(ctx context.Context, config, uid string) {
				service.NotifyConfigUpdate(ctx, config, uid)
				cloudInit.Update(ctx)
			})

			// We want to inspect the tasks databases even if those calls fail.
			_ = c.SetUserLandscapeConfig(ctx, tc.conf)
			_ = c.SetLandscapeAgentUID(ctx, "landscapeUID")

			// There is no direct way to observe the result of that function other than relying on the implementation details of the task database.
			tasksFiles, err := filepath.Glob(filepath.Join(storageDir, "*.tasks"))
			require.NoError(t, err, "NotifyConfigUpdate: could not list the tasks files storage dir: %s", storageDir)
			require.NotEmpty(t, tasksFiles, "NotifyConfigUpdate: should have created a tasks file")

			b, err := os.ReadFile(tasksFiles[0])
			require.NoError(t, err, "NotifyConfigUpdate: should have caused creation of a tasks file")
			task := string(b)
			require.NotEmpty(t, task, "NotifyConfigUpdate: tasks file should not be empty")

			require.Contains(t, task, tasks.LandscapeConfigure{}.String(), "NotifyConfigUpdate: tasks file should contain a LandscapeConfigure task")

			basepath := testutils.TestFixturePath(t)

			wantTask := testutils.LoadWithUpdateFromGolden(t, task, testutils.WithGoldenPath(filepath.Join(basepath, "golden.tasks")))
			require.Equal(t, wantTask, task, "NotifyConfigUpdate: tasks file should contain the expected Landscape client config")

			agentYaml, err := os.ReadFile(filepath.Join(homedir, ".cloud-init", "agent.yaml"))
			require.NoError(t, err, "NotifyConfigUpdate: could not read agent.yaml")
			wantAgentYaml := testutils.LoadWithUpdateFromGolden(t, string(agentYaml), testutils.WithGoldenPath(filepath.Join(basepath, "agent.yaml")))
			require.YAMLEq(t, wantAgentYaml, string(agentYaml), "NotifyConfigUpdate: tasks file and agent.yaml don't match")
		})
	}
}
func executeLandscapeConfigTemplate(t *testing.T, in string, certPath string, url net.Addr) string {
	t.Helper()

	tmpl := template.Must(template.New(t.Name()).Parse(in))

	data := struct {
		CertPath, HostURL string
	}{
		CertPath: certPath,
		HostURL:  url.String(),
	}

	out := bytes.Buffer{}
	err := tmpl.Execute(&out, data)
	require.NoError(t, err, "Setup: could not generate Landscape config from template")

	return out.String()
}

//nolint:revive // Context goes after testing.T
func setUpLandscapeMock(t *testing.T, ctx context.Context, addr string, certPath string, grpcErr error) (lis net.Listener, server *grpc.Server, service *landscapemockservice.Service) {
	t.Helper()

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", addr)
	require.NoError(t, err, "Setup: can't listen")

	var opts []grpc.ServerOption
	if certPath != "" {
		cert := filepath.Join(certPath, "cert.pem")
		key := filepath.Join(certPath, "key.pem")

		serverCert, err := tls.LoadX509KeyPair(cert, key)
		require.NoError(t, err, "Setup: could not load Landscape mock server credentials")

		config := &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientAuth:   tls.NoClientCert,
			MinVersion:   tls.VersionTLS12,
		}

		opts = append(opts, grpc.Creds(credentials.NewTLS(config)))
	}

	var logs bytes.Buffer
	h := slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})
	service = landscapemockservice.New(landscapemockservice.WithLogger(slog.New(h)), landscapemockservice.WithConnectError(grpcErr))

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}

		// Cannot use t.Log outside the main goroutine
		log.Printf("Landscape server logs:\n%s", logs.String())
	})

	server = grpc.NewServer(opts...)
	landscapeapi.RegisterLandscapeHostAgentServer(server, service)

	return lis, server, service
}

type mockCloudInit struct {
	writeCalled   atomic.Bool
	removeCalled  atomic.Bool
	instanceIDSet atomic.Bool

	writeErr  bool
	removeErr bool
}

func (c *mockCloudInit) WriteDistroData(distroName string, cloudInit string, requestID string) error {
	c.writeCalled.Store(true)
	c.instanceIDSet.Store(requestID != "")

	if c.writeErr {
		return errors.New("could not write distro cloud-init data: mock error")
	}

	return nil
}

func (c *mockCloudInit) RemoveDistroData(distroName string) error {
	c.removeCalled.Store(true)

	if c.removeErr {
		return errors.New("could not remove distro cloud-init data: mock error")
	}

	return nil
}

type mockConfig struct {
	proToken              string
	landscapeClientConfig string
	landscapeAgentUID     string

	proTokenErr        bool
	landscapeConfigErr bool
	landscapeUIDErr    bool
	setLandscapeUIDErr bool

	mu sync.Mutex
}

func (m *mockConfig) LandscapeClientConfig() (string, config.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.landscapeConfigErr {
		return "", config.SourceNone, errors.New("Mock error")
	}
	return m.landscapeClientConfig, config.SourceUser, nil
}

func (m *mockConfig) Subscription() (string, config.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proTokenErr {
		return "", config.SourceNone, errors.New("Mock error")
	}
	if m.proToken == "" {
		return "", config.SourceNone, nil
	}
	return m.proToken, config.SourceUser, nil
}

func (m *mockConfig) LandscapeAgentUID() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.landscapeUIDErr {
		return "", errors.New("Mock error")
	}
	return m.landscapeAgentUID, nil
}

func (m *mockConfig) SetLandscapeAgentUID(ctx context.Context, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.setLandscapeUIDErr {
		return errors.New("Mock error")
	}

	m.landscapeAgentUID = uid
	return nil
}
