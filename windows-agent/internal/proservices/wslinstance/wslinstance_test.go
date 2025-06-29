package wslinstance_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/wslinstance"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	task.Register[testTask]()

	m.Run()
}

func TestServe(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		dontRegister       bool
		dontSendDistroName bool

		skipConnectedHandshake bool
		skipProHandshake       bool
		skipLandscapeHandshake bool

		duplicateStream bool

		wantNeverInDatabase         bool
		wantConnectionNeverAttached bool
	}{
		"Success": {},

		// Partial failure: only one stream connects
		"Error when two streams connect under the same name": {duplicateStream: true},

		// Early failure: before/during add to database
		"Error when the distro name is not sent":            {dontSendDistroName: true, wantNeverInDatabase: true},
		"Error when the distro does not exist":              {dontRegister: true, wantNeverInDatabase: true},
		"Error when Connected never performs the handshake": {skipConnectedHandshake: true, wantNeverInDatabase: true},

		// Late failure: during wait for other streams
		"Error when Pro never performs the handshake":       {skipProHandshake: true, wantConnectionNeverAttached: true},
		"Error when Landscape never performs the handshake": {skipLandscapeHandshake: true, wantConnectionNeverAttached: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty database")

			landscape := &landscapeCtlMock{}

			service := wslinstance.New(ctx, db, landscape)
			server := grpc.NewServer()
			agentapi.RegisterWSLInstanceServer(server, service)

			lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
			require.NoError(t, err, "Setup: could not listen to dynamically-allocated port")
			defer lis.Close()

			var wg sync.WaitGroup
			wg.Add(1)
			defer wg.Wait()
			go func() {
				defer wg.Done()
				err := server.Serve(lis)
				if err != nil {
					t.Logf("Serve exited with error: %v", err)
				}
			}()
			defer server.Stop()

			distroName := wsltestutils.RandomDistroName(t)
			if !tc.dontRegister {
				distroName, _ = wsltestutils.RegisterDistro(t, ctx, false)
			}

			sendName := distroName
			if tc.dontSendDistroName {
				sendName = ""
			}

			wps := newMockWSLProService(t, ctx, mockWslProServiceOptions{
				address:    lis.Addr().String(),
				distroName: sendName,

				noHandshakeConnected:         tc.skipConnectedHandshake,
				noHandshakeProCommands:       tc.skipProHandshake,
				noHandshakeLandscapeCommands: tc.skipLandscapeHandshake,
			})
			defer wps.Stop()

			timeout := time.Minute
			if tc.wantNeverInDatabase {
				wps.requireDone(t, timeout, "did not disconnect before adding the distro to the database")
				require.Empty(t, db.GetAll(), "No distro should have been added to the database")
				return
			}
			require.Eventually(t, func() bool {
				_, ok := db.Get(distroName)
				return ok
			}, timeout, time.Second, "Distro was never added to database")

			if tc.duplicateStream {
				wps2 := newMockWSLProService(t, ctx, mockWslProServiceOptions{
					address:    lis.Addr().String(),
					distroName: distroName,
				})
				defer wps2.Stop()

				time.Sleep(time.Second)

				err := wps2.connStream.Send(&agentapi.DistroInfo{WslName: distroName})
				require.Error(t, err, "Second stream should have errored")

				err = wps2.proStream.Send(&agentapi.MSG{Data: &agentapi.MSG_WslName{WslName: distroName}})
				require.Error(t, err, "Second stream should have errored")

				err = wps2.lpeStream.Send(&agentapi.MSG{Data: &agentapi.MSG_WslName{WslName: distroName}})
				require.Error(t, err, "Second stream should have errored")
			}

			require.Eventually(t, func() bool {
				return landscape.updateCount.Load() > 0
			}, 10*time.Second, time.Second, "Landscape was never notified")

			if tc.wantConnectionNeverAttached {
				wps.requireDone(t, timeout, "did not disconnect before assigning a connection")
				d, _ := db.Get(distroName)
				conn, err := d.Connection()
				require.NoError(t, err, "Connection should return no error")
				require.Nil(t, conn, "Distro should not have been assigned a connection")
				return
			}
			require.Eventually(t, func() bool {
				d, _ := db.Get(distroName)
				conn, err := d.Connection()
				if err != nil {
					return false
				}
				return conn != nil
			}, timeout, time.Second, "Distro never got assigned a connection")

			wps.sendInfo(t, &agentapi.DistroInfo{
				WslName:            distroName,
				Id:                 "TEST_ID",
				VersionId:          "TEST_VERSION_ID",
				PrettyName:         "TEST_PRETTY_NAME",
				ProAttached:        true,
				Hostname:           "TEST_HOSTNAME",
				CreatedByLandscape: true,
			})

			require.Eventually(t, func() bool {
				return landscape.updateCount.Load() > 1
			}, 10*time.Second, time.Second, "Landscape was never notified after sending info")

			d, _ := db.Get(distroName)
			props := d.Properties()
			require.Equal(t, "TEST_ID", props.DistroID, "Mismatch between sent and stored properties")
			require.Equal(t, "TEST_VERSION_ID", props.VersionID, "Mismatch between sent and stored properties")
			require.Equal(t, "TEST_PRETTY_NAME", props.PrettyName, "Mismatch between sent and stored properties")
			require.True(t, props.ProAttached, "Mismatch between sent and stored properties")
			require.Equal(t, "TEST_HOSTNAME", props.Hostname, "Mismatch between sent and stored properties")
		})
	}
}

func TestSendCommands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	db, err := database.New(ctx, t.TempDir())
	require.NoError(t, err, "Setup: could not create empty database")

	landscape := &landscapeCtlMock{}

	service := wslinstance.New(ctx, db, landscape)
	server := grpc.NewServer()
	agentapi.RegisterWSLInstanceServer(server, service)

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	require.NoError(t, err, "Setup: could not listen to dynamically-allocated port")
	defer lis.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		err := server.Serve(lis)
		if err != nil {
			t.Logf("Serve exited with error: %v", err)
		}
	}()
	defer server.Stop()

	distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

	wps := newMockWSLProService(t, ctx, mockWslProServiceOptions{
		address:    lis.Addr().String(),
		distroName: distroName,
	})
	defer wps.Stop()

	timeout := time.Minute

	require.Eventually(t, func() bool {
		d, ok := db.Get(distroName)
		if !ok {
			return false
		}
		conn, err := d.Connection()
		if err != nil {
			return false
		}
		return conn != nil
	}, timeout, time.Second, "Distro never got assigned a connection")

	distro, ok := db.Get(distroName)
	require.True(t, ok, "Distro should not be removed from the database")

	conn, err := distro.Connection()
	require.NoError(t, err, "distro.Connection should return no error")
	require.NotNil(t, conn, "Connection should not have been nil")

	err = conn.SendProAttachment("hello123")
	require.NoError(t, err, "SendProAttachment should return no error")

	err = conn.SendProAttachment("MOCK_ERROR")
	require.Error(t, err, "SendProAttachment should have returned an error")

	err = conn.SendLandscapeConfig("hello=world")
	require.NoError(t, err, "SendLandscapeConfig should return no error")

	err = conn.SendLandscapeConfig("MOCK_ERROR")
	require.Error(t, err, "SendLandscapeConfig should have returned an error")

	wps.Stop()

	err = conn.SendProAttachment("hello123")
	require.Error(t, err, "SendProAttachment should return an error after disconnecting")

	err = conn.SendLandscapeConfig("hello123")
	require.Error(t, err, "SendLandscapeConfig should return an error after disconnecting")
}

// landscapeCtlMock mocks the landscape client.
//
// disconnected and err are inputs to manipulate mock behaviour.
// updateCount is used to assert that the SendUpdatedInfo function has been called.
type landscapeCtlMock struct {
	disconnected bool
	err          bool

	updateCount atomic.Int32
}

func (c *landscapeCtlMock) SendUpdatedInfo(ctx context.Context) error {
	c.updateCount.Add(1)

	if c.disconnected {
		return errors.New("Sending updated info to disconnected landscape")
	}

	if c.err {
		return errors.New("mock error")
	}
	return nil
}

// mockWSLProService mocks the actions performed by the Linux-side client and services.
type mockWSLProService struct {
	connStream agentapi.WSLInstance_ConnectedClient
	proStream  agentapi.WSLInstance_ProAttachmentCommandsClient
	lpeStream  agentapi.WSLInstance_LandscapeConfigCommandsClient

	cancel  func()
	conn    *grpc.ClientConn
	running sync.WaitGroup
}

type mockWslProServiceOptions struct {
	address    string
	distroName string

	noHandshakeConnected         bool
	noHandshakeProCommands       bool
	noHandshakeLandscapeCommands bool
}

// newMockWSLProService creates a wslDistroMock, establishing a connection to the control stream.
//
//nolint:revive // testing.T should go before context, regardless of what these linters say.
func newMockWSLProService(t *testing.T, ctx context.Context, opt mockWslProServiceOptions) (mock *mockWSLProService) {
	t.Helper()

	mock = &mockWSLProService{}

	conn, err := grpc.NewClient(opt.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "wslDistroMock: could not setup a control address client")

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	mock.conn = conn
	mock.cancel = cancel

	c := agentapi.NewWSLInstanceClient(conn)

	mock.connStream, err = c.Connected(ctx)
	require.NoError(t, err, "wslDistroMock: could not connect to Connected stream")

	if !opt.noHandshakeConnected {
		err = mock.connStream.Send(&agentapi.DistroInfo{WslName: opt.distroName})
		require.NoError(t, err, "wslDistroMock: could not send info via Connected stream")
	}

	mock.proStream, err = c.ProAttachmentCommands(ctx)
	require.NoError(t, err, "wslDistroMock: could not connect to ProAttachmentCommands stream")
	if !opt.noHandshakeProCommands {
		err = sendWslName(mock.proStream.Send, opt.distroName)
		require.NoError(t, err, "wslDistroMock: could not send wsl name via ProAttachmentCommands stream")
	}

	mock.lpeStream, err = c.LandscapeConfigCommands(ctx)
	require.NoError(t, err, "wslDistroMock: could not connect to LandscapeConfigCommands stream")
	if !opt.noHandshakeLandscapeCommands {
		err = sendWslName(mock.lpeStream.Send, opt.distroName)
		require.NoError(t, err, "wslDistroMock: could not send wsl name via LandscapeConfigCommands stream")
	}

	mock.running.Add(2)
	go mock.replyProAttachmentCommands(t)
	go mock.replyLandscapeConfigCommands(t)

	return mock
}

func sendWslName(send func(*agentapi.MSG) error, wslName string) error {
	return send(&agentapi.MSG{
		Data: &agentapi.MSG_WslName{
			WslName: wslName,
		},
	})
}

func sendResult(send func(*agentapi.MSG) error, result error) error {
	var errMsg string
	if result != nil {
		errMsg = result.Error()
	}

	return send(&agentapi.MSG{
		Data: &agentapi.MSG_Result{
			Result: errMsg,
		},
	})
}

// Stop stops the Linux-side service.
func (m *mockWSLProService) Stop() {
	m.cancel()
	m.conn.Close()
	m.running.Wait()
}

func (m *mockWSLProService) requireDone(t *testing.T, timeout time.Duration, msg string, args ...any) {
	t.Helper()

	ch := make(chan struct{})
	go func() {
		m.running.Wait()
		close(ch)
	}()

	select {
	case <-ch:
		return
	case <-time.After(timeout):
	}

	require.Failf(t, "WSL Pro service was not done", msg, args...)
}

func (m *mockWSLProService) replyProAttachmentCommands(t *testing.T) {
	t.Helper()
	defer m.running.Done()
	defer m.cancel()

	for {
		msg, err := m.proStream.Recv()
		if err != nil {
			log.Warningf("%s: Could not receive pro command: %v", t.Name(), err)
			return
		}

		var send error
		if msg.GetToken() == "MOCK_ERROR" {
			send = errors.New("mock error")
		}

		err = sendResult(m.proStream.Send, send)
		if err != nil {
			log.Warningf("%s: Could not send pro command result: %v", t.Name(), err)
			m.Stop()
			return
		}
	}
}

func (m *mockWSLProService) replyLandscapeConfigCommands(t *testing.T) {
	t.Helper()
	defer m.running.Done()
	defer m.cancel()

	for {
		msg, err := m.lpeStream.Recv()
		if err != nil {
			log.Warningf("%s: Could not receive Landscape command: %v", t.Name(), err)
			return
		}

		var send error
		if msg.GetConfig() == "MOCK_ERROR" {
			send = errors.New("mock error")
		}

		err = sendResult(m.lpeStream.Send, send)
		if err != nil {
			log.Warningf("%s: Could not send Landscape command result: %v", t.Name(), err)
			m.Stop()
			return
		}
	}
}

// sendInfo sends the specified info from the Linux-side client to the wslinstance service.
func (m *mockWSLProService) sendInfo(t *testing.T, info *agentapi.DistroInfo) {
	t.Helper()

	err := m.connStream.Send(info)
	require.NoError(t, err, "wslDistroMock SendInfo expected no errors")
}

type testTask struct {
	ID string
}

var completedTeskTasks = testutils.NewSet[string]()

func (t testTask) Execute(ctx context.Context, _ task.Connection) error {
	completedTeskTasks.Set(t.ID)
	return nil
}

func (t testTask) String() string {
	return fmt.Sprintf("Test task with ID %s", t.ID)
}
