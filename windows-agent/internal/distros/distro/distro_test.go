package distro_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/worker"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

// globalStartupMu protects against multiple distros starting at the same time.
var globalStartupMu sync.Mutex

// startupMutex exists so that all distro tests share the same startup mutex.
// This mutex prevents multiple distros from starting at the same time, which
// could freeze the machine.
//
// When a mock WSL is used, this concern does not exist so we provide a new
// mutex for every test so they can run in parallel without interference.
func startupMutex() *sync.Mutex {
	if wsl.MockAvailable() {
		// No real distros: use a different mutex every test
		return &sync.Mutex{}
	}

	// Real distros: use a the same mutex for all tests
	return &globalStartupMu
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	registeredDistro, registeredGUID := wsltestutils.RegisterDistro(t, ctx, false)
	_, anotherRegisteredGUID := wsltestutils.RegisterDistro(t, ctx, false)
	nonRegisteredDistro, fakeGUID := wsltestutils.NonRegisteredDistro(t)

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
		nilMutex               bool

		wantErr     bool
		wantErrType error
	}{
		"Success with a registered distro":              {distro: registeredDistro},
		"Success with a registered distro and its GUID": {distro: registeredDistro, withGUID: registeredGUID},

		// Error cases
		"Error when a constructing a distro with another distro's GUID": {distro: nonRegisteredDistro, withGUID: anotherRegisteredGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when a registered distro with a wrong GUID":              {distro: registeredDistro, withGUID: fakeGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when the distro is not registered":                       {distro: nonRegisteredDistro, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when the distro is not registered, but the GUID is":      {distro: nonRegisteredDistro, withGUID: registeredGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when neither the distro nor the GUID are registered":     {distro: nonRegisteredDistro, withGUID: fakeGUID, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error when the startup mutex is nil":                           {distro: registeredDistro, nilMutex: true, wantErr: true},
		"Error when the distro working directory cannot be created":     {distro: nonRegisteredDistro, preventWorkDirCreation: true, wantErr: true, wantErrType: &distro.NotValidError{}},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var d *distro.Distro
			var err error

			var args []distro.Option
			if tc.withGUID != "" {
				GUID, err := uuid.Parse(tc.withGUID)
				require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
				args = append(args, distro.WithGUID(GUID))
			}

			workDir := t.TempDir()
			if tc.preventWorkDirCreation {
				workDir = filepath.Join(workDir, "workdir")
				err := os.WriteFile(workDir, []byte("I'm here to interfere"), 0600)
				require.NoError(t, err, "Setup: could not write file to interfere with distro's MkDir")
			}

			mu := startupMutex()
			if tc.nilMutex {
				mu = nil
			}

			d, err = distro.New(ctx, tc.distro, props, t.TempDir(), mu, args...)
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

	name, guid := wsltestutils.RegisterDistro(t, ctx, false)

	GUID, err := uuid.Parse(guid)
	require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)

	d, err := distro.New(ctx, name, distro.Properties{}, t.TempDir(), startupMutex(), distro.WithGUID(GUID))
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

	distro1, guid1 := wsltestutils.RegisterDistro(t, ctx, false)
	_, guid2 := wsltestutils.RegisterDistro(t, ctx, false)
	nonRegisteredDistro, fakeGUID := wsltestutils.NonRegisteredDistro(t)

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
		t.Run(name, func(t *testing.T) {
			// Create an always valid distro
			d, err := distro.New(ctx, distro1, distro.Properties{}, t.TempDir(), startupMutex())
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
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			dname, _ := wsltestutils.RegisterDistro(t, ctx, false)
			d, err := distro.New(ctx, dname, props1, t.TempDir(), startupMutex())
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

func TestLockReleaseAwake(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	const wslSleepDelay = 8 * time.Second

	testCases := map[string]struct {
		// Breaking lock
		unregisterDistro        bool
		invalidateBeforeLock    bool
		invalidateBeforeRelease bool
		errorOnLock             bool
		unregisterDistroLate    bool

		// Stacking
		doubleLock               bool
		stopDistroInbetweenLocks bool
		errorOnSecondLock        bool
		errorStateOnSecondLock   bool

		// Alternatives to Release
		cleanupDistro bool

		// Backend
		mockOnly bool

		wantLockErr       bool
		wantSecondLockErr bool
		wantReleaseErr    bool
	}{
		"Registered distro is kept awake until ReleaseAwake":                              {},
		"Registered distro is kept awake until ReleaseAwake (two locks and two releases)": {doubleLock: true},
		"Registered distro is awaken by second LockAwake":                                 {doubleLock: true, stopDistroInbetweenLocks: true},

		"Registered distro is kept awake until distro cleanup": {cleanupDistro: true},

		"Error on invalidated distro before Lock":    {invalidateBeforeLock: true, wantLockErr: true},
		"Error on invalidated distro before Release": {invalidateBeforeRelease: true, wantReleaseErr: true},
		"Error on uregistered distro":                {unregisterDistro: true, wantLockErr: true},

		// Mocked errors
		"Error due to inability to start distro":                  {mockOnly: true, errorOnLock: true, wantLockErr: true},
		"Error due to inability to get state in second LockAwake": {mockOnly: true, doubleLock: true, stopDistroInbetweenLocks: true, errorStateOnSecondLock: true, wantSecondLockErr: true},
		"Error due to inability to start distro a second time":    {mockOnly: true, doubleLock: true, stopDistroInbetweenLocks: true, errorOnSecondLock: true, wantSecondLockErr: true},
		"Error when the distro is unregistered under the hood":    {mockOnly: true, unregisterDistroLate: true, wantLockErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var mock *wslmock.Backend
			if wsl.MockAvailable() {
				t.Parallel()
				mock = wslmock.New()
				mock.WslLaunchInteractiveError = tc.errorOnLock
				ctx = wsl.WithMock(ctx, mock)
			} else if tc.mockOnly {
				t.Skip("This test is only available for the mock back-end")
			}

			// That makes the mock break at Touch().
			distroName := wsltestutils.RandomDistroName(t) + "unregistered-late"
			if tc.unregisterDistroLate {
				_ = wsltestutils.RegisterDistroNamed(t, ctx, distroName)
			} else {
				distroName, _ = wsltestutils.RegisterDistro(t, ctx, true)
			}

			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), startupMutex())
			defer d.Cleanup(context.Background())

			require.NoError(t, err, "Setup: distro New should return no error")

			wsltestutils.TerminateDistro(t, ctx, distroName)

			if tc.invalidateBeforeLock {
				d.Invalidate(ctx)
			}
			if tc.unregisterDistro {
				wsltestutils.UnregisterDistro(t, ctx, distroName)
			}

			// Start distro
			err = d.LockAwake()
			if tc.wantLockErr {
				require.Errorf(t, err, "LockAwake should have returned an error")
				state := wsltestutils.DistroState(t, ctx, distroName)
				require.NotEqualf(t, "Running", state, "distro should not run when LockAwake fails")

				return
			}
			require.NoErrorf(t, err, "LockAwake should have returned no error")

			require.Eventually(t, func() bool {
				return wsltestutils.DistroState(t, ctx, distroName) == "Running"
			}, 10*time.Second, time.Second, "distro should have started after calling LockAwake")

			// Second lock
			if tc.doubleLock {
				if tc.stopDistroInbetweenLocks {
					wsltestutils.TerminateDistro(t, ctx, distroName)
				}

				if tc.errorOnSecondLock {
					mock.WslLaunchInteractiveError = true
				}

				if tc.errorStateOnSecondLock {
					mock.StateError = true
				}

				err = d.LockAwake()
				if tc.wantSecondLockErr {
					require.Errorf(t, err, "Second LockAwake should have returned an error")
					return
				}
				require.NoErrorf(t, err, "Second LockAwake should have returned no error")

				require.Eventually(t, func() bool {
					d := wsl.NewDistro(ctx, distroName)
					state, err := d.State()
					if err != nil {
						t.Logf("d.State returned error: %v", err)
						return false
					}
					return state == wsl.Running
				}, wslSleepDelay+2*time.Second, time.Second, "distro should have started after calling LockAwake")
			}

			time.Sleep(wslSleepDelay + 2*time.Second)

			require.Equal(t, "Running", wsltestutils.DistroState(t, ctx, distroName), "LockAwake should have kept the distro running")

			// Stopping distro
			if tc.cleanupDistro {
				// Method 1: Cleanup
				d.Cleanup(ctx)
			} else {
				// Method 2: ReleaseAwake
				if tc.invalidateBeforeRelease {
					d.Invalidate(ctx)
				}

				err = d.ReleaseAwake()
				if tc.wantReleaseErr {
					require.Error(t, err, "ReleaseAwake should return an error")
					return
				}
				require.NoError(t, err, "ReleaseAwake should return no error")

				if tc.doubleLock {
					time.Sleep(wslSleepDelay + 2*time.Second)
					require.Equal(t, "Running", wsltestutils.DistroState(t, ctx, distroName), "Distro should stay awake after two calls to LockAwake and only one to ReleaseAwake")

					// Need two releases
					err = d.ReleaseAwake()
					require.NoError(t, err, "ReleaseAwake should return no error")
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
			}, wslSleepDelay+2*time.Second, time.Second, "distro should have stopped after calling ReleaseAwake due to inactivity.")

			// Try one more ReleaseAwake than needed
			err = d.ReleaseAwake()
			require.Error(t, err, "ReleaseAwake should return and error when called more times than LockAwake")
		})
	}
}

func TestNoSimultaneousStartups(t *testing.T) {
	t.Parallel()

	if !wsl.MockAvailable() {
		t.Skip("Skipped without mocks to avoid messing with the global mutex")
	}

	ctx := wsl.WithMock(context.Background(), wslmock.New())
	var startupMu sync.Mutex

	distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
	d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), &startupMu)
	defer d.Cleanup(context.Background())
	require.NoError(t, err, "Setup: distro New should return no error")

	wsltestutils.TerminateDistro(t, ctx, distroName)

	// Lock the startup mutex to pretend some other distro is starting up
	const lockAwakeMaxTime = 20 * time.Second
	ch := make(chan error)

	func() {
		startupMu.Lock()
		defer startupMu.Unlock()

		go func() {
			// We send the error to be asserted in the main goroutine because
			// failed assertions outside the test goroutine cause panics.
			ch <- d.LockAwake()
			close(ch)
		}()

		time.Sleep(lockAwakeMaxTime)
		state := wsltestutils.DistroState(t, ctx, distroName)
		require.Equal(t, "Stopped", state, "Distro should not start while the mutex is locked")
	}()

	// The startup mutex has been released to pretend some other distro finished starting up

	select {
	case <-time.After(lockAwakeMaxTime):
		require.Fail(t, "LockAwake should have returned after releasing the startup mutex")
	case err := <-ch:
		require.NoError(t, err, "LockAwake should return no error")
		break
	}

	state := wsltestutils.DistroState(t, ctx, distroName)
	require.Equal(t, "Running", state, "Distro should start after the mutex is released")
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
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()

				mock := wslmock.New()
				mock.StateError = tc.wslError
				ctx = wsl.WithMock(ctx, mock)
			}

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, true)
			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), startupMutex())
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

	distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		constructorReturnErr bool

		wantErr bool
	}{
		"Success when worker construction succeeds": {},
		"Error when worker construction fails":      {constructorReturnErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			type testContextMarker int
			ctx := context.WithValue(ctx, testContextMarker(42), 27)

			withMockWorker, worker := mockWorkerInjector(tc.constructorReturnErr)

			workDir := t.TempDir()

			d, err := distro.New(ctx,
				distroName,
				distro.Properties{},
				workDir,
				startupMutex(),
				distro.WithTaskProcessingContext(ctx),
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
		})
	}
}

func TestInvalidateIdempotent(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

	inj, w := mockWorkerInjector(false)

	d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), &globalStartupMu, inj)
	defer d.Cleanup(context.Background())
	require.NoError(t, err, "Setup: distro New should return no error")

	require.True(t, d.IsValid(), "successfully constructed distro should be valid")

	d.Invalidate(ctx)
	require.False(t, d.IsValid(), "distro should stop being valid after calling invalidate")
	require.False(t, (*w).stopCalled, "worker Stop should only be called during cleanup")

	(*w).stopCalled = false

	d.Invalidate(ctx)
	require.False(t, d.IsValid(), "distro should stop being valid after calling invalidate")
	require.False(t, (*w).stopCalled, "worker Stop should not be called in subsequent invalidations")

	d.Invalidate(ctx)
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

	distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		function      string // What method to call
		invalidDistro bool   // Whether to use an invalid distro
		nilArg        bool   // If the function takes an argument other than a context, nil will be used

		wantErr          bool
		wantWorkerCalled bool
	}{
		"IsActive succeeds":                 {function: "IsActive", wantWorkerCalled: true},
		"IsActive errors on invalid distro": {function: "IsActive", invalidDistro: true, wantErr: true},

		"Connection succeeds":                 {function: "Connection", wantWorkerCalled: true},
		"Connection errors on invalid distro": {function: "Connection", invalidDistro: true, wantErr: true},

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
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inj, w := mockWorkerInjector(false)

			d, err := distro.New(ctx, distroName, distro.Properties{}, t.TempDir(), startupMutex(), inj)
			defer d.Cleanup(context.Background())
			require.NoError(t, err, "Setup: distro New should return no error")

			if tc.invalidDistro {
				d.Invalidate(ctx)
			}

			worker := *w
			var funcCalled bool

			switch tc.function {
			case "IsActive":
				_, err = d.IsActive()
				funcCalled = worker.isActiveCalled

			case "Connection":
				_, err = d.Connection()
				funcCalled = worker.connectionCalled

			case "SetConnection":
				err = d.SetConnection(&mockConnection{})
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

func TestUninstall(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		unregisterDistro bool
		mockErr          bool

		wantErr bool
	}{
		"Success": {},

		"Error when the distro is not registered": {unregisterDistro: true, wantErr: true},
		"Error when uninstalling fails":           {mockErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				m := wslmock.New()
				m.WslUnregisterDistributionError = tc.mockErr
				ctx = wsl.WithMock(ctx, m)
				defer m.ResetErrors()
			} else if tc.mockErr {
				t.Skip("This test is only available with the WSL mock")
			}

			name, _ := wsltestutils.RegisterDistro(t, ctx, false)

			d, err := distro.New(ctx, name, distro.Properties{}, t.TempDir(), startupMutex())
			require.NoError(t, err, "Setup: distro New should return no errors")

			if tc.unregisterDistro {
				d := wsl.NewDistro(ctx, name)
				err := d.Unregister()
				require.NoError(t, err, "Setup: could not unregister distro")
			}

			err = d.Uninstall(ctx)
			if tc.wantErr {
				require.Error(t, err, "Uninstall should have returned an error")
				return
			}
			require.NoError(t, err, "Uninstall should return no error")
		})
	}
}

type mockWorker struct {
	newCtx    context.Context
	newDistro *distro.Distro
	newDir    string

	isActiveCalled      bool
	connectionCalled    bool
	setConnectionCalled bool
	submitTasksCalled   bool
	stopCalled          bool
}

func mockWorkerInjector(constructorReturnsError bool) (distro.Option, **mockWorker) {
	mock := new(*mockWorker)
	newMockWorker := func(ctx context.Context, d *distro.Distro, tmpDir string) (distro.Worker, error) {
		w := &mockWorker{
			newCtx:    ctx,
			newDistro: d,
			newDir:    tmpDir,
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

func (w *mockWorker) Connection() worker.Connection {
	w.connectionCalled = true
	return nil
}

func (w *mockWorker) SetConnection(conn worker.Connection) {
	w.setConnectionCalled = true
}

func (w *mockWorker) SubmitTasks(...task.Task) error {
	w.submitTasksCalled = true
	return nil
}

func (w *mockWorker) SubmitDeferredTasks(...task.Task) error {
	return nil
}

func (w *mockWorker) EnqueueDeferredTasks() {
	panic("Not implemented")
}

func (w *mockWorker) Stop(context.Context) {
	w.stopCalled = true
}

type mockConnection struct{}

func (c *mockConnection) SendProAttachment(proToken string) error {
	return nil
}

func (c *mockConnection) SendLandscapeConfig(lpeConfig string) error {
	return nil
}

func (c *mockConnection) Close() {
}
