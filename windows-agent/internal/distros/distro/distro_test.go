package distro_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/worker"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	registeredDistro, registeredGUID := testutils.RegisterDistro(t, ctx, false)
	_, anotherRegisteredGUID := testutils.RegisterDistro(t, ctx, false)
	nonRegisteredDistro, fakeGUID := testutils.NonRegisteredDistro(t)

	props := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "100.04",
		PrettyName:  "Ubuntu 100.04.0 LTS",
		ProAttached: true,
		Hostname:    "testMachine",
	}

	testCases := map[string]struct {
		distro                 string
		withGUID               string
		preventWorkDirCreation bool
		withProvisioning       bool

		wantErr     bool
		wantErrType error
	}{
		"Success with a registered distro":                   {distro: registeredDistro},
		"Success with a registered distro and its GUID":      {distro: registeredDistro, withGUID: registeredGUID},
		"Success with a registered distro with provisioning": {distro: registeredDistro, withProvisioning: true},

		// Error cases
		"Error when workdir cannot be created":                          {distro: registeredDistro, preventWorkDirCreation: true, wantErr: true},
		"Error when a constructing a distro with another distro's GUID": {distro: nonRegisteredDistro, withGUID: anotherRegisteredGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when a registered distro with a wrong GUID":              {distro: registeredDistro, withGUID: fakeGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when the distro is not registered":                       {distro: nonRegisteredDistro, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when the distro is not registered, but the GUID is":      {distro: nonRegisteredDistro, withGUID: registeredGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when neither the distro nor the GUID are registered":     {distro: nonRegisteredDistro, withGUID: fakeGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var d *distro.Distro
			var err error

			var args []distro.Option
			if tc.withGUID != "" {
				GUID, err := uuid.Parse(tc.withGUID)
				require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
				args = append(args, distro.WithGUID(GUID))
			}

			if tc.withProvisioning {
				args = append(args, distro.WithProvisioning(&mockProvisioning{}))
			}

			workDir := t.TempDir()
			if tc.preventWorkDirCreation {
				workDir = filepath.Join(workDir, "workdir")
				err := os.WriteFile(workDir, []byte("I'm here to interfere"), 0600)
				require.NoError(t, err, "Setup: could not write file to interfere with distro's MkDir")
			}

			d, err = distro.New(ctx, tc.distro, props, workDir, args...)
			defer d.Cleanup(context.Background())

			if tc.wantErr {
				require.Error(t, err, "New() should have returned an error")
				if tc.wantErrType != nil {
					require.ErrorIsf(t, err, tc.wantErrType, "New() should have returned an error of type %T", tc.wantErrType)
				}
				return
			}

			require.NoError(t, err, "New() should have returned no error")
			require.Equal(t, tc.distro, d.Name(), "distro.Name should match the one it was constructed with")
			require.Equal(t, registeredGUID, d.GUID(), "distro.GUID should match the one it was constructed with")
			require.Equal(t, props, d.Properties(), "distro.Properties should match the one it was constructed with because they were never directly modified")
		})
	}
}

func TestString(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	name, guid := testutils.RegisterDistro(t, ctx, false)

	GUID, err := uuid.Parse(guid)
	require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
	d, err := distro.New(ctx, name, distro.Properties{}, t.TempDir(), distro.WithGUID(GUID))
	defer d.Cleanup(context.Background())

	require.NoError(t, err, "Setup: unexpected error in distro.New")

	s := d.String()
	require.Contains(t, s, name, "String() should contain the name of the distro")
	require.Contains(t, s, guid, "String() should contain the GUID of the distro")
}

func TestIsValid(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, guid1 := testutils.RegisterDistro(t, ctx, false)
	_, guid2 := testutils.RegisterDistro(t, ctx, false)
	nonRegisteredDistro, fakeGUID := testutils.NonRegisteredDistro(t)

	testCases := map[string]struct {
		distro string
		guid   string

		want bool
	}{
		"True with a registered distro and its GUID": {distro: distro1, guid: guid1, want: true},

		// Invalid cases
		"False with a registered distro another distro's GUID":               {distro: distro1, guid: guid2, want: false},
		"False with a registered distro with a fake GUID":                    {distro: distro1, guid: fakeGUID, want: false},
		"False with a non-registered distro with a registered distro's GUID": {distro: nonRegisteredDistro, guid: guid1, want: false},
		"False with a non-registered distro with a fake GUID":                {distro: nonRegisteredDistro, guid: fakeGUID, want: false},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Create an always valid distro
			d, err := distro.New(ctx, distro1, distro.Properties{}, t.TempDir())
			defer d.Cleanup(context.Background())

			require.NoError(t, err, "Setup: distro New() should return no errors")

			// Change values and assert on IsValid
			d.GetIdentity().Name = tc.distro

			GUID, err := uuid.Parse(tc.guid)
			require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
			d.GetIdentity().GUID = GUID

			got := d.IsValid()
			require.Equal(t, tc.want, got, "IsValid should return expected value")
		})
	}
}

func TestSetProperties(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	props1 := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "100.04",
		PrettyName:  "Ubuntu 100.04.0 LTS",
		ProAttached: true,
	}

	props2 := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "200.04",
		PrettyName:  "Ubuntu 200.04.0 LTS",
		ProAttached: false,
	}

	testCases := map[string]struct {
		sameProps bool

		want bool
	}{
		"Return true when setting a new set of properties":     {want: true},
		"Return false when setting the same set of properties": {sameProps: true, want: false},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			dname, _ := testutils.RegisterDistro(t, ctx, false)
			d, err := distro.New(ctx, dname, props1, t.TempDir())
			require.NoError(t, err, "Setup: distro New should return no errors")

			p := props2
			if tc.sameProps {
				p = props1
			}

			got := d.SetProperties(p)
			require.Equal(t, tc.want, got, "Unexpected return value from SetProperties")
		})
	}
}

func TestPushPopAwake(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	const wslSleepDelay = 8 * time.Second

	testCases := map[string]struct {
		// Breaking push
		unregisterDistro     bool
		invalidateBeforePush bool
		invalidateBeforePop  bool
		errorOnKeepAwake     bool

		// Stacking
		doublePush                bool
		stopDistroInbetweenPushes bool
		errorOnSecondKeepAwake    bool
		errorStateOnSecondPush    bool

		// Alternatives to Pop
		cleanupDistro bool

		// Backend
		mockOnly bool

		wantPushErr       bool
		wantSecondPushErr bool
		wantPopErr        bool
	}{
		"Registered distro is kept awake until PopAwake":                       {},
		"Registered distro is kept awake until PopAwake (two pushes and pops)": {doublePush: true},
		"Registered distro is awaken by second PushAwake":                      {doublePush: true, stopDistroInbetweenPushes: true},

		"Registered distro is kept awake until distro cleanup": {cleanupDistro: true},

		"Error on invalidated distro before Push": {invalidateBeforePush: true, wantPushErr: true},
		"Error on invalidated distro before Pop":  {invalidateBeforePop: true, wantPopErr: true},
		"Error on uregistered distro":             {unregisterDistro: true, wantPushErr: true},

		// Mocked errors
		"Error due to inability to start distro":                  {mockOnly: true, errorOnKeepAwake: true, wantPushErr: true},
		"Error due to inability to get state in second PushAwake": {mockOnly: true, doublePush: true, stopDistroInbetweenPushes: true, errorStateOnSecondPush: true, wantSecondPushErr: true},
		"Error due to inability to start distro a second time":    {mockOnly: true, doublePush: true, stopDistroInbetweenPushes: true, errorOnSecondKeepAwake: true, wantSecondPushErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var mock *wslmock.Backend
			if wsl.MockAvailable() {
				t.Parallel()
				mock = wslmock.New()
				mock.WslLaunchInteractiveError = tc.errorOnKeepAwake
				ctx = wsl.WithMock(ctx, mock)
			} else {
				if tc.mockOnly {
					t.Skip("This test is only available for the mock back-end")
				}
			}

			distroName, _ := testutils.RegisterDistro(t, ctx, true)

			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir())
			defer d.Cleanup(context.Background())

			require.NoError(t, err, "Setup: distro New should return no error")

			testutils.TerminateDistro(t, ctx, distroName)

			if tc.invalidateBeforePush {
				d.Invalidate(errors.New("setup: invalidating distro"))
			}
			if tc.unregisterDistro {
				testutils.UnregisterDistro(t, ctx, distroName)
			}

			// Start distro
			err = d.PushAwake()
			if tc.wantPushErr {
				require.Errorf(t, err, "PushAwake should have returned an error")
				state := testutils.DistroState(t, ctx, distroName)
				require.NotEqualf(t, "Running", state, "distro should not run when PushAwake fails")

				return
			}
			require.NoErrorf(t, err, "PushAwake should have returned no error")

			require.Eventually(t, func() bool {
				return testutils.DistroState(t, ctx, distroName) == "Running"
			}, 10*time.Second, time.Second, "distro should have started after calling PushAwake")

			// Second push
			if tc.doublePush {
				if tc.stopDistroInbetweenPushes {
					testutils.TerminateDistro(t, ctx, distroName)
				}

				if tc.errorOnSecondKeepAwake {
					mock.WslLaunchInteractiveError = true
				}

				if tc.errorStateOnSecondPush {
					mock.StateError = true
				}

				err = d.PushAwake()
				if tc.wantSecondPushErr {
					require.Errorf(t, err, "Second PushAwake should have returned an error")
					return
				}
				require.NoErrorf(t, err, "Second PushAwake should have returned no error")

				require.Eventually(t, func() bool {
					return testutils.DistroState(t, ctx, distroName) == "Running"
				}, 10*time.Second, time.Second, "distro should have started after calling PushAwake")
			}

			time.Sleep(2 * wslSleepDelay)

			require.Equal(t, "Running", testutils.DistroState(t, ctx, distroName), "PushAwake should have kept the distro running")

			if tc.cleanupDistro {
				d.Cleanup(ctx)
			} else {
				if tc.invalidateBeforePop {
					d.Invalidate(errors.New("distro invalidated by test"))
				}

				err = d.PopAwake()
				if tc.wantPopErr {
					require.Error(t, err, "PopAwake should return an error")
					return
				}
				require.NoError(t, err, "PopAwake should return no error")

				if tc.doublePush {
					time.Sleep(wslSleepDelay + 2*time.Second)
					require.Equal(t, "Running", testutils.DistroState(t, ctx, distroName), "Distro should stay awake after two calls to PushAwake and only one to PopAwake")

					// Need two pops
					err = d.PopAwake()
					require.NoError(t, err, "PopAwake should return no error")
				}
			}

			require.Eventually(t, func() bool {
				d := wsl.NewDistro(ctx, distroName)
				state, err := d.State()
				if err != nil {
					t.Logf("d.State returned error: %v", err)
					return false
				}
				return state == wsl.Stopped
			}, 2*wslSleepDelay, time.Second, "distro should have stopped after calling PopAwake due to inactivity.")

			err = d.PopAwake()
			require.Error(t, err, "PopAwake should return and error when called more times than PushAwake")
		})
	}
}

func TestState(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		unregister bool
		stop       bool
		wslError   bool

		want    wsl.State
		wantErr bool
	}{
		"Success with a running distro": {want: wsl.Running},
		"Success with a stopped distro": {stop: true, want: wsl.Stopped},

		"Error on unregistered distro":  {unregister: true, wantErr: true},
		"Error due to WSL erroring out": {wslError: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()

				mock := wslmock.New()
				mock.StateError = tc.wslError
				ctx = wsl.WithMock(ctx, mock)
			}

			distroName, _ := testutils.RegisterDistro(t, ctx, true)
			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir())
			require.NoError(t, err, "Setup: distro New should return no errors")

			gowslDistro := wsl.NewDistro(ctx, distroName)
			out, err := gowslDistro.Command(ctx, "exit 0").CombinedOutput()
			require.NoError(t, err, "Setup: could not start WSL distro (%v): %s", err, string(out))

			if tc.stop {
				err := gowslDistro.Terminate()
				require.NoError(t, err, "Setup: could not terminate: %v", err)
			}

			if tc.unregister {
				err := gowslDistro.Unregister()
				require.NoError(t, err, "Setup: could not unregister: %v", err)
			}

			got, err := d.State()
			if tc.wantErr {
				require.Error(t, err, "expected distro.State to return an error")
				return
			}

			require.NoError(t, err, "expected distro.State to return no error")
			require.Equal(t, tc.want, got, "Mismatch between expected and reported states")
		})
	}
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestWorkerConstruction(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distroName, _ := testutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		constructorReturnErr bool

		wantErr bool
	}{
		"Success when worker construction succeeds": {},
		"Error when worker construction fails":      {constructorReturnErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			type testContextMarker int
			ctx := context.WithValue(ctx, testContextMarker(42), 27)

			withMockWorker, worker := mockWorkerInjector(tc.constructorReturnErr)

			workDir := t.TempDir()
			provisioning := mockProvisioning{}

			d, err := distro.New(ctx,
				distroName,
				distro.Properties{},
				workDir,
				distro.WithTaskProcessingContext(ctx),
				distro.WithProvisioning(provisioning),
				withMockWorker)
			defer d.Cleanup(context.Background())

			if tc.wantErr {
				require.Error(t, err, "distro New should return an error when the worker construction errors out")
				return
			}
			require.NoError(t, err, "distro New should return no error")

			require.NotNil(t, *worker, "Worker's constructor should be called in the distro's constructor")
			require.NotNil(t, (*worker).newCtx.Value(testContextMarker(42)), "Worker's constructor should be called with the distro's context or a child of it")
			require.Equal(t, d, (*worker).newDistro, "Worker's constructor should be called with the distro it is attached to")
			require.Equal(t, workDir, (*worker).newDir, "Worker's constructor should be called with the same workdir as the distro's")
			require.Equal(t, provisioning, (*worker).newProvisioning, "Worker's constructor should be called with the config passed to the distro")
		})
	}
}

func TestInvalidateIdempotent(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distroName, _ := testutils.RegisterDistro(t, ctx, false)

	inj, w := mockWorkerInjector(false)

	d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), inj)
	defer d.Cleanup(context.Background())
	require.NoError(t, err, "Setup: distro New should return no error")

	require.True(t, d.IsValid(), "successfully constructed distro should be valid")

	d.Invalidate(errors.New("Hi! I'm an error"))
	require.False(t, d.IsValid(), "distro should stop being valid after calling invalidate")
	require.False(t, (*w).stopCalled, "worker Stop should only be called during cleanup")

	(*w).stopCalled = false

	d.Invalidate(errors.New("Hi! I'm another error"))
	require.False(t, d.IsValid(), "distro should stop being valid after calling invalidate")
	require.False(t, (*w).stopCalled, "worker Stop should not be called in subsequent invalidations")

	d.Invalidate(errors.New("Hi! I'm another error"))
	require.False(t, d.IsValid(), "distro should stop being valid after calling invalidate")
	require.False(t, (*w).stopCalled, "worker Stop should not be called in subsequent invalidations")
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestWorkerWrappers(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distroName, _ := testutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		function      string // What method to call
		invalidDistro bool   // Whether to use an invalid distro
		nilArg        bool   // If the function takes an argument other than a context, nil will be used

		wantErr          bool
		wantWorkerCalled bool
	}{
		"IsActive succeeds":                 {function: "IsActive", wantWorkerCalled: true},
		"IsActive errors on invalid distro": {function: "IsActive", invalidDistro: true, wantErr: true},

		"Client succeeds":                 {function: "Client", wantWorkerCalled: true},
		"Client errors on invalid distro": {function: "Client", invalidDistro: true, wantErr: true},

		"SetConnection succeeds":                                       {function: "SetConnection", wantWorkerCalled: true},
		"SetConnection succeeds with nil connection":                   {function: "SetConnection", nilArg: true, wantWorkerCalled: true},
		"SetConnection succeeds with nil connection on invalid distro": {function: "SetConnection", nilArg: true, wantWorkerCalled: true},
		"SetConnection errors on invalid distro":                       {function: "SetConnection", invalidDistro: true, wantErr: true},

		"SubmitTasks succeeds with zero tasks": {function: "SubmitTasks", nilArg: true, wantWorkerCalled: true},
		"SubmitTasks succeeds with arguments":  {function: "SubmitTasks", wantWorkerCalled: true},
		"SubmitTasks errors on invalid distro": {function: "SubmitTasks", invalidDistro: true, wantErr: true},

		"Stop succeeds":                 {function: "Stop", wantWorkerCalled: true},
		"Stop errors on invalid distro": {function: "Stop", invalidDistro: true, wantWorkerCalled: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inj, w := mockWorkerInjector(false)

			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), inj)
			defer d.Cleanup(context.Background())
			require.NoError(t, err, "Setup: distro New should return no error")

			if tc.invalidDistro {
				d.Invalidate(errors.New("test invalidation"))
			}

			worker := *w
			var funcCalled bool

			switch tc.function {
			case "IsActive":
				_, err = d.IsActive()
				funcCalled = worker.isActiveCalled

			case "Client":
				_, err = d.Client()
				funcCalled = worker.clientCalled

			case "SetConnection":
				var conn *grpc.ClientConn
				if !tc.nilArg {
					conn = &grpc.ClientConn{}
				}
				err = d.SetConnection(conn)
				funcCalled = worker.setConnectionCalled

			case "SubmitTasks":
				var t []task.Task
				if !tc.nilArg {
					t = make([]task.Task, 5)
				}
				err = d.SubmitTasks(t...)
				funcCalled = worker.submitTasksCalled

			case "Stop":
				d.Cleanup(context.Background())
				funcCalled = worker.stopCalled
				err = nil
			default:
				require.Fail(t, "Setup: Unexpected tc.function")
			}

			if tc.wantErr {
				require.Error(t, err, "function %q should have returned an error when the distro is invalid", tc.function)
			} else {
				require.NoError(t, err, "function %q should have returned no error when the distro is valid", tc.function)
			}

			if tc.wantWorkerCalled {
				require.True(t, funcCalled, "Worker function should have been called")
			} else {
				require.False(t, funcCalled, "Worker function should not have been called")
			}
		})
	}
}

type mockWorker struct {
	newCtx          context.Context
	newDistro       *distro.Distro
	newDir          string
	newProvisioning worker.Provisioning

	isActiveCalled      bool
	clientCalled        bool
	setConnectionCalled bool
	submitTasksCalled   bool
	stopCalled          bool
}

func mockWorkerInjector(constructorReturnsError bool) (distro.Option, **mockWorker) {
	mock := new(*mockWorker)
	newMockWorker := func(ctx context.Context, d *distro.Distro, tmpDir string, conf worker.Provisioning) (distro.Worker, error) {
		w := &mockWorker{
			newCtx:          ctx,
			newDistro:       d,
			newDir:          tmpDir,
			newProvisioning: conf,
		}
		*mock = w
		if constructorReturnsError {
			return nil, errors.New("test error")
		}
		return w, nil
	}

	return distro.WithNewWorker(newMockWorker), mock
}

func (w *mockWorker) IsActive() bool {
	w.isActiveCalled = true
	return false
}

func (w *mockWorker) Client() wslserviceapi.WSLClient {
	w.clientCalled = true
	return nil
}

func (w *mockWorker) SetConnection(conn *grpc.ClientConn) {
	w.setConnectionCalled = true
}

func (w *mockWorker) SubmitTasks(...task.Task) error {
	w.submitTasksCalled = true
	return nil
}

func (w *mockWorker) Stop(context.Context) {
	w.stopCalled = true
}

type mockProvisioning struct{}

func (c mockProvisioning) ProvisioningTasks(ctx context.Context) ([]task.Task, error) {
	return nil, nil
}
