package landscape_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/golden"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/landscape/landscapemockservice"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

const defaultLandscapeConfig = `
[host]
url = "{{ .HostURL }}"

[client]
account_name = testuser
registration_key = password1
`

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

		landscapeUIDReadErr  bool
		landscapeUIDWriteErr bool

		emptyToken bool
		tokenErr   bool

		requireCertificate         bool
		breakLandscapeClientConfig bool

		breakUIDFile bool
		uid          string

		wantErr           bool
		wantDistroSkipped bool
	}{
		"Success":                         {},
		"Success in non-first contact":    {uid: "123"},
		"Success with an SSL certificate": {requireCertificate: true},

		"Error when the context is cancelled before Connected": {precancelContext: true, wantErr: true},
		"Error when the config is empty":                       {wantErr: true},
		"Error when the landscape URL cannot be retrieved":     {wantErr: true},
		"Error when the landscape UID cannot be retrieved":     {landscapeUIDReadErr: true, wantErr: true},
		"Error when the landscape UID cannot be stored":        {landscapeUIDWriteErr: true, wantErr: true},
		"Error when the server cannot be reached":              {serverNotAvailable: true, wantErr: true},
		"Error when the first-contact SendUpdatedInfo fails":   {tokenErr: true, wantErr: true},
		"Error when the config cannot be accessed":             {breakLandscapeClientConfig: true, wantErr: true},
		"Error when the config cannot be parsed":               {wantErr: true},
		"Error when the SSL certificate cannot be read":        {wantErr: true},
		"Error when the SSL certificate is not valid":          {wantErr: true},
		"Error when there is no Ubuntu Pro token":              {emptyToken: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			p := ""
			if tc.requireCertificate {
				p = certPath
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", p)
			defer lis.Close()

			conf := &mockConfig{
				proToken: "TOKEN",

				// We trigger an error on first-contact SendUpdatedInfo by erroring out in conf.ProToken()
				proTokenErr: tc.tokenErr,

				// We trigger errors trying to read or write to/from the registry
				landscapeUIDErr:    tc.landscapeUIDReadErr,
				setLandscapeUIDErr: tc.landscapeUIDWriteErr,

				// We trigger an error when deciding to use a certificate or not
				landscapeConfigErr: tc.breakLandscapeClientConfig,

				landscapeAgentUID: tc.uid,
			}

			if tc.emptyToken {
				conf.proToken = ""
			}

			lconf := defaultLandscapeConfig
			if fixture, err := os.ReadFile(filepath.Join(golden.TestFixturePath(t), "landscape.conf")); err != nil {
				require.ErrorIs(t, err, os.ErrNotExist, "Setup: could not load landscape config")
				// Fixture does not exist: use base Landcape confing
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

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
			_, err = db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")

			service, err := landscape.New(ctx, conf, db)
			require.NoError(t, err, "Setup: NewClient should return no errrors")

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

			require.True(t, service.Connected(), "Connected should have returned false after succeeding to connect")

			require.Eventually(t, func() bool {
				return len(mockService.MessageLog()) > 0
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

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
			} else if tc.stateErr {
				t.Skip("This test is skipped because it necessitates the GoWSL mock")
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", "")

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr()),
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
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

			service, err := landscape.New(ctx, conf, db, landscape.WithHostname(hostname))
			require.NoError(t, err, "Landscape NewClient should not return an error")

			ctl := service.Controller()

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
				return len(mockService.MessageLog()) > 0
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a message from the client")

			messages := mockService.MessageLog()
			require.Len(t, messages, 1, "Exactly one message should've been sent to Landscape")
			msg := &messages[0] // Pointer to avoid copying mutex

			assert.Empty(t, msg.UID, "First UID received by the server should be empty")
			assert.Equal(t, wantAccountName, msg.AccountName, "Mismatch between local account name and that received by the server")
			assert.Equal(t, wantRegistrationKey, msg.RegistrationKey, "Mismatch between local registration key and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")

			if tc.wantDistroSkipped {
				require.Empty(t, msg.Instances, "No distro should've been sent to Landscape")
			} else {
				require.Len(t, msg.Instances, 1, "Exactly one distro should've been sent to Landscape")
				got := msg.Instances[0]
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

			if !tc.distroIsRunning {
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
				return len(mockService.MessageLog()) > 1
			}, 10*time.Second, 100*time.Millisecond, "Landscape server should receive a second message from the client")

			messages = mockService.MessageLog()
			require.Len(t, messages, 2, "Exactly two messages should've been sent to Landscape")
			msg = &messages[1] // Pointer to avoid copying mutex

			assertHasPrefix(t, wantUIDprefix, msg.UID, "Mismatch between local host ID and that received by the server")
			assert.Equal(t, wantAccountName, msg.AccountName, "Mismatch between local account name and that received by the server")
			assert.Equal(t, wantRegistrationKey, msg.RegistrationKey, "Mismatch between local registration key and that received by the server")
			assert.Equal(t, wantHostname, msg.Hostname, "Mismatch between local host hostname and that received by the server")
			assert.Equal(t, wantHostToken, msg.Token, "Mismatch between local host pro token and those received by the server")
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				mock := wslmock.New()
				ctx = wsl.WithMock(ctx, mock)
			}

			lis, server, mockService := setUpLandscapeMock(t, ctx, "localhost:", "")
			defer lis.Close()
			defer server.Stop()

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr()),
			}

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			const hostname = "HOSTNAME"

			service, err := landscape.New(ctx, conf, db, landscape.WithHostname(hostname))
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

			for uid := range hosts {
				err = mockService.Disconnect(uid)
				require.NoError(t, err, "Server should not fail to disconnect the client")
			}

			// Fast tickrate to ensure we detect the disconnection. The client reconnects after 1s so we should always win.
			const tick = 10 * time.Millisecond
			require.Eventually(t, func() bool {
				return !service.Connected()
			}, 5*time.Second, tick, "Client should have disconnected after the stream is dropped")

			// Detecting reconnection
			require.Eventually(t, func() bool {
				return service.Connected()
			}, 5*time.Second, 100*time.Millisecond, "Client should have reconnected after the stream is dropped")

			server.Stop()
			require.Eventually(t, func() bool {
				return !service.Connected()
			}, 5*time.Second, 100*time.Millisecond, "Client should have disconnected after the server is stopped")

			// Restart server at the same address
			lis, server, _ = setUpLandscapeMock(t, ctx, lis.Addr().String(), "")
			defer lis.Close()

			//nolint:errcheck // We don't care
			go server.Serve(lis)
			defer server.Stop()

			require.Eventually(t, func() bool {
				return service.Connected()
			}, 5*time.Second, 500*time.Millisecond, "Client should have reconnected after restarting the server")
		})
	}
}

func TestReconnect(t *testing.T) {
	t.Parallel()

	requestReconnect := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		s.Controller().Reconnect(ctx)
	}

	changeAddress := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		// We change the address to an equivalent one, so that the reconnect is triggered abd the connection succeeds
		c.mu.Lock()
		fmt.Println(c.landscapeClientConfig)
		c.landscapeClientConfig = strings.ReplaceAll(c.landscapeClientConfig, "127.0.0.1", "localhost")
		fmt.Println(c.landscapeClientConfig)
		c.mu.Unlock()

		c.triggerNotifications()
	}

	changeCertificate := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		// We change the path to an equivalent one, so that the reconnect is triggered and the connection still succeeds
		const sep = filepath.Separator
		from := fmt.Sprintf("%c", sep)       // from: /
		to := fmt.Sprintf("%c.%c", sep, sep) // to:   /./

		c.mu.Lock()
		c.landscapeClientConfig = strings.Replace(c.landscapeClientConfig, from, to, 1)
		c.mu.Unlock()

		c.triggerNotifications()
	}

	changeIrrelevant := func(ctx context.Context, s *landscape.Service, c *mockConfig) {
		c.mu.Lock()
		c.landscapeClientConfig = c.landscapeClientConfig + "\n[exta]\ninfo=this section does not matter"
		c.mu.Unlock()

		c.triggerNotifications()
	}

	testCases := map[string]struct {
		useCertificate bool
		trigger        func(context.Context, *landscape.Service, *mockConfig)

		wantNoReconnect       bool
		wantImmediateRconnect bool
	}{
		"Reconnect when explicitly requesting a reconnection": {trigger: requestReconnect, wantImmediateRconnect: true},
		"Reconnect when changing the URL":                     {trigger: changeAddress},
		"Reconnect when changing the certificate path":        {trigger: changeCertificate, useCertificate: true},
		"Don't reconnect when changing irrelevant config":     {trigger: changeIrrelevant, wantNoReconnect: true},
	}

	for name, tc := range testCases {
		tc := tc
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
			}

			lis, server, _ := setUpLandscapeMock(t, ctx, "localhost:", certPath)

			conf := &mockConfig{
				proToken:              "TOKEN",
				landscapeClientConfig: executeLandscapeConfigTemplate(t, lcapeConfig, certPath, lis.Addr()),
			}

			//nolint:errcheck // We don't care about these errors
			go server.Serve(lis)
			defer server.Stop()

			db, err := database.New(ctx, t.TempDir(), conf)
			require.NoError(t, err, "Setup: database New should not return an error")

			service, err := landscape.New(ctx, conf, db)
			require.NoError(t, err, "Setup: New should not return an error")

			err = service.Connect()
			require.NoError(t, err, "Setup: Connect should not return an error")

			require.Eventually(t, func() bool {
				return service.Connected()
			}, 5*time.Second, 500*time.Millisecond, "Client should have reconnected after restarting the server")

			ch := make(chan struct{})
			go func() {
				tc.trigger(ctx, service, conf)
				close(ch)
			}()

			fmt.Println("AAAAAAAAA")

			const timeout = 10 * time.Second
			const tickRate = 100 * time.Millisecond
			if tc.wantNoReconnect {
				requireAlways(t, service.Connected, timeout, tickRate, "Client should not have disconnected")
				return
			}

			require.Eventually(t, func() bool { return !service.Connected() },
				timeout, tickRate, "Client should have disconnected during reconnection")

			select {
			case <-ch:
			case <-time.After(20 * time.Second):
				require.Fail(t, "Reconnect should have returned")
			}

			if tc.wantImmediateRconnect {
				require.True(t, service.Connected(), "Client should have connected before returning")
				return
			}

			require.Eventually(t, service.Connected,
				20*time.Second, time.Second, "Client should have connected after reconnection")
		})
	}
}

func requireAlways(t *testing.T, predicate func() bool, waitFor time.Duration, tick time.Duration, msgAndArgs ...interface{}) {
	t.Helper()

	tk := time.NewTicker(tick)
	defer tk.Stop()

	tm := time.NewTimer(waitFor)
	defer tm.Stop()

	for {
		select {
		case <-tm.C:
			return
		case <-tk.C:
		}

		require.True(t, predicate(), msgAndArgs...)
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
func setUpLandscapeMock(t *testing.T, ctx context.Context, addr string, certPath string) (lis net.Listener, server *grpc.Server, service *landscapemockservice.Service) {
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

	server = grpc.NewServer(opts...)
	service = landscapemockservice.New()
	landscapeapi.RegisterLandscapeHostAgentServer(server, service)

	return lis, server, service
}

type mockConfig struct {
	proToken              string
	landscapeClientConfig string
	landscapeAgentUID     string

	proTokenErr        bool
	landscapeConfigErr bool
	landscapeUIDErr    bool
	setLandscapeUIDErr bool

	callbacks []func()

	mu sync.Mutex
}

func (m *mockConfig) LandscapeClientConfig(ctx context.Context) (string, config.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.landscapeConfigErr {
		return "", config.SourceNone, errors.New("Mock error")
	}
	return m.landscapeClientConfig, config.SourceUser, nil
}

func (m *mockConfig) ProvisioningTasks(ctx context.Context, distroName string) ([]task.Task, error) {
	return nil, nil
}

func (m *mockConfig) Subscription(ctx context.Context) (string, config.Source, error) {
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

func (m *mockConfig) LandscapeAgentUID(ctx context.Context) (string, error) {
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

	defer m.triggerNotifications()

	m.landscapeAgentUID = uid
	return nil
}

func (m *mockConfig) Notify(f func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = append(m.callbacks, f)
}

func (m *mockConfig) triggerNotifications() {
	for _, f := range m.callbacks {
		go f()
	}
}
