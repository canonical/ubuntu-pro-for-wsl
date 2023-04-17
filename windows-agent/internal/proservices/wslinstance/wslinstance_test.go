package wslinstance_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/wslinstance"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, err := database.New(ctx, t.TempDir(), nil)
	require.NoError(t, err, "Setup: empty database New() should return no error")
	defer db.Close(ctx)

	_, err = wslinstance.New(context.Background(), db)
	require.NoError(t, err, "New should never return an error")
}

type step int

const (
	never step = iota
	beforeLinuxServe
	beforeSendInfo
	afterSendInfo
	afterDatabaseQuery
	afterDistroShouldBeActive
	beforeSecondSendInfo
	afterSecondSendInfo
	afterPropertiesRefreshed
)

func (w step) String() string {
	switch w {
	case never:
		return "never"
	case beforeLinuxServe:
		return "beforeLinuxServe"
	case beforeSendInfo:
		return "beforeSendInfo"
	case afterSendInfo:
		return "afterSendInfo"
	case afterDatabaseQuery:
		return "afterDatabaseQuery"
	case afterDistroShouldBeActive:
		return "afterDistroShouldBeActive"
	case beforeSecondSendInfo:
		return "beforeSecondSendInfo"
	case afterSecondSendInfo:
		return "afterSecondSendInfo"
	case afterPropertiesRefreshed:
		return "afterPropertiesRefreshed"
	}
	return fmt.Sprintf("Unknown when (%d)", int(w))
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestConnected(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distroName, _ := testutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		useEmptyDistroName  bool
		stopLinuxSideClient step
		sendSecondInfo      bool
		skipLinuxServe      bool

		wantDone step
		wantErr  bool
	}{
		"Successful connection with WSL distro":      {},
		"Successful connection and property refresh": {sendSecondInfo: true},

		"Error on never serving on Linux":              {skipLinuxServe: true, wantDone: afterDistroShouldBeActive, wantErr: true},
		"Error on disconnect before send info":         {stopLinuxSideClient: beforeLinuxServe, wantDone: beforeLinuxServe, wantErr: true},
		"Error with blank distro name":                 {useEmptyDistroName: true, wantDone: afterSendInfo, wantErr: true},
		"Error when it cannot send the port to distro": {stopLinuxSideClient: afterSendInfo, wantDone: afterSendInfo, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			distroName := distroName
			if tc.useEmptyDistroName {
				distroName = ""
			}

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			defer db.Close(ctx)

			srv, err := newWrappedService(ctx, db)
			require.NoError(t, err, "Setup: wslinstance New() should never return an error")

			grpcServer, ctrlAddr := serveWSLInstance(t, ctx, srv)
			defer grpcServer.Stop()

			wsl := newWslDistroMock(t, ctx, ctrlAddr)
			defer wsl.stopClient()

			// WSL-side server is not serving yet.
			now := beforeLinuxServe
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			wantErrNeverReceivePort := tc.wantDone < afterDatabaseQuery && tc.wantDone != never
			if !tc.skipLinuxServe {
				go wsl.serve(t, wantErrNeverReceivePort)
				defer wsl.stopServer()
			}

			// WSL-side server is serving, but no info has been sent yet.
			now = beforeSendInfo
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			// Simulate Linux-side client sending its info.
			info := &agentapi.DistroInfo{
				WslName:     distroName,
				Id:          "ubuntu",
				VersionId:   "22.04",
				PrettyName:  "Ubuntu 22.04.1 LTS",
				ProAttached: false,
			}
			wsl.sendInfo(t, info)

			// WSL-side server is serving, info was sent.
			now = afterSendInfo
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			// Distro should eventually appear in the database.
			var d *distro.Distro
			require.Eventuallyf(t, func() (ok bool) {
				d, ok = db.Get(distroName)
				return ok
			}, time.Second, 10*time.Millisecond, "Distro %q should be added to the database after sending its info", distroName)

			// Ensure we got matching properties on the agent side.
			props := propsFromInfo(t, info)
			require.Equal(t, props, d.Properties(), "Distro properties should match those sent via the SendInfo.")

			// Connected has added the distro to the database.
			now = afterDatabaseQuery
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			// Small amount of time to mitigate races
			const epsilon = 100 * time.Millisecond

			// newWslServiceConn has a 2 second timeout with 5 retries
			const maxDelay = 5*2*time.Second + epsilon

			if tc.skipLinuxServe {
				// Distro should not become active: there is no service on Linux to connect to.
				time.Sleep(maxDelay)
				active, err := d.IsActive()
				require.NoError(t, err, "IsActive should return no error as the distro should still be valid")
				require.False(t, active, "Distro should never become active if there is no Linux-side service to connect to")
			} else {
				// Distro should become active (establish a connection to the Linux-side service).
				require.Eventually(t, func() bool {
					v, err := d.IsActive()
					require.NoError(t, err, "IsActive should return no error as the distro should still be valid")
					return v
				}, maxDelay, 10*time.Millisecond,
					"Distro should become active after sending its info for the first time")
			}

			// The distro has had its stream attached.
			now = afterDistroShouldBeActive
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			if !tc.sendSecondInfo {
				return
			}

			// Send new info with changing parameter.
			info.ProAttached = true
			wsl.sendInfo(t, info)

			// We have sent info for a second time
			now = afterSecondSendInfo
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			if continueTest := checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv); !continueTest {
				return
			}

			// One of the property should have changed.
			props = propsFromInfo(t, info)
			require.Eventually(t, func() bool {
				return d.Properties() == props
			}, time.Second, 10*time.Millisecond, "Distro properties should be refreshed after every call to SendInfo to the control stream")

			// The database has been updated after the second info
			now = afterPropertiesRefreshed
			stopWSLClientOnMatchingStep(tc.stopLinuxSideClient, now, wsl)
			checkConnectedStatus(t, tc.wantDone, tc.wantErr, now, srv)
		})
	}
}

// testLoggerInterceptor replaces the logging middleware by printing the return
// error of Connected to the test Log.
//
//nolint:thelper // The logs would be reported to come from the entrails of the GRPC module. It's more helpful to reference this function to see that it is the middleware reporting.
func testLoggerInterceptor(t *testing.T) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := handler(srv, stream); err != nil {
			fmt.Printf("Connected returned error: %v\n", err)
		}
		fmt.Println("Connected returned with no error")
		return nil
	}
}

// wrappedService is a wrapper around the tested wslinstance.Service in order to
// get some information about what and when Connected() returns.
type wrappedService struct {
	wslinstance.Service
	Errch chan error
}

// newWrappedService is a wrapper around wslinstance.New. It initializes the monitoring
// around the service.
func newWrappedService(ctx context.Context, db *database.DistroDB) (s wrappedService, err error) {
	inst, err := wslinstance.New(ctx, db)
	return wrappedService{
		Service: inst,
		Errch:   make(chan error),
	}, err
}

// Connected is a wrapper around wslinstance.Connected.
func (s *wrappedService) Connected(stream agentapi.WSLInstance_ConnectedServer) error {
	err := s.Service.Connected(stream)
	s.Errch <- err
	return err
}

// wait waits until the function Connected has returned.
// - if ok is true, returnErr is the return value of Connected.
// - if ok is false, the wait times out hence Connected has not returned yet. returnedErr is therefore not valid.
//
//nolint:revive // Returning the error as first argument is strange but it makes sense here, we mimic the (value, ok) return type of a map access.
func (s *wrappedService) wait(timeout time.Duration) (returnedErr error, connectedHasReturned bool) {
	select {
	case returnedErr = <-s.Errch:
		return returnedErr, true
	case <-time.After(timeout):
		return nil, false
	}
}

//nolint:revive // testing.T should go before context, I won't listen to anyone arguing the contrary.
func serveWSLInstance(t *testing.T, ctx context.Context, srv wrappedService) (server *grpc.Server, address string) {
	t.Helper()

	server = grpc.NewServer(grpc.StreamInterceptor(testLoggerInterceptor(t)))
	agentapi.RegisterWSLInstanceServer(server, &srv)

	t.Logf("serveWSLInstance: selecting port")

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp4", "localhost:")
	require.NoError(t, err, "Setup: could not listen to autoselected port")

	t.Logf("serveWSLInstance: serving on: %v", lis.Addr().String())

	go func() { _ = server.Serve(lis) }()

	return server, lis.Addr().String()
}

// wslDistroMock mocks the actions performed by the Linux-side client and services.
type wslDistroMock struct {
	grpcServer *grpc.Server
	ctrlStream agentapi.WSLInstance_ConnectedClient

	clientStop func()
}

// newWslDistroMock creates a wslDistroMock, establishing a connection to the control stream.
//
//nolint:revive // testing.T should go before context, regardless of what these linters say.
func newWslDistroMock(t *testing.T, ctx context.Context, ctrlAddr string) (mock *wslDistroMock) {
	t.Helper()

	mock = &wslDistroMock{
		grpcServer: grpc.NewServer(),
	}

	ctrlConn, err := grpc.DialContext(ctx, ctrlAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "wslDistroMock: could not dial control address")

	ctx, cancel := context.WithCancel(ctx)
	mock.clientStop = cancel

	c := agentapi.NewWSLInstanceClient(ctrlConn)
	mock.ctrlStream, err = c.Connected(ctx)
	require.NoError(t, err, "wslDistroMock: could not connect to control stream")

	return mock
}

// serve starts the Linux-side service. It is an unimplemented service that exists
// just so the wslinstance can connect to it, but any GRPC call will cause an error.
//
// wantErrNeverReceivePort is a test parameter for when you expect the Linux endpoint
// of the control stream to never receive the port.
func (m *wslDistroMock) serve(t *testing.T, wantErrNeverReceivePort bool) {
	t.Helper()

	msg, err := m.ctrlStream.Recv()
	if wantErrNeverReceivePort {
		require.Error(t, err, "wslDistroMock should not receive the port to listen to from the control stream")
		return
	}
	require.NoError(t, err, "wslDistroMock should have received the port to listen to from the control stream")

	t.Logf("wslDistroMock: Received msg: %v", msg)

	p := msg.GetPort()
	require.NotEqual(t, 0, p, "Received invalid port :0 from server")

	// Create our service
	addr := fmt.Sprintf("localhost:%d", p)
	lis, err := net.Listen("tcp4", addr)
	require.NoError(t, err, "wslDistroMock: startServe: could not listen to %q", addr)

	t.Logf("wslDistroMock: Listening to: %s", addr)

	wslserviceapi.RegisterWSLServer(m.grpcServer, &wslserviceapi.UnimplementedWSLServer{})

	_ = m.grpcServer.Serve(lis)
}

// sendInfo sends the specified info from the Linux-side client to the wslinstance service.
func (m *wslDistroMock) sendInfo(t *testing.T, info *agentapi.DistroInfo) {
	t.Helper()

	err := m.ctrlStream.Send(info)
	require.NoError(t, err, "wslDistroMock SendInfo expected no errors")
}

// stopServer stops the Linux-side service.
func (m *wslDistroMock) stopServer() {
	m.grpcServer.Stop()
}

// stopServe stops the Linux-side service.
func (m *wslDistroMock) stopClient() {
	m.clientStop()
}

// stopWSLClientOnMatchingStep stops the Linux-side client if wantStopStep is the same as currentStep.
func stopWSLClientOnMatchingStep(wantStopStep, currentStep step, wsl *wslDistroMock) {
	if currentStep == wantStopStep {
		wsl.stopClient()
	}
}

// checkConnectedStatus has two options:
//   - if wantDoneStep != currentStep: assert that wslservice.Connected has not yet returned.
//   - otherwise, assert that it has returned, and that its return value matches wantErr.
func checkConnectedStatus(t *testing.T, wantDoneStep step, wantErr bool, currentStep step, srv wrappedService) (continueTest bool) {
	t.Helper()

	connectedErr, stopped := srv.wait(100 * time.Millisecond)
	if currentStep != wantDoneStep {
		require.False(t, stopped, "Connect() function should still be running at step %q but is has now stopped (should stop at step %q)", currentStep, wantDoneStep)
		return true
	}

	require.True(t, stopped, "Connect() function should have stopped at step %q", currentStep)

	if wantErr {
		require.Error(t, connectedErr, "Connect() should return an error at step %q", currentStep)
		return false
	}
	require.NoError(t, connectedErr, "Connect() should return no error at step %q", currentStep)

	return false
}

// propsFromInfo converts a DistroInfo object into a Properties, failing the test in case of error.
func propsFromInfo(t *testing.T, info *agentapi.DistroInfo) distro.Properties {
	t.Helper()
	props, err := wslinstance.PropsFromInfo(info)
	require.NoErrorf(t, err, "PropsFromInfo should not return any error. Info: %#v", info)
	return props
}
