package worker_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/worker"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func init() {
	task.Register[emptyTask]()
}

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	t.Parallel()

	type taskFileState int
	const (
		fileNotExist taskFileState = iota
		fileIsEmpty
		fileHasOneTask
		fileHasTwoTasks

		fileHasBadSyntax
		fileHasNonRegisteredTask
		fileIsDir
	)

	testCases := map[string]struct {
		taskFile    taskFileState
		fillUpQueue bool

		wantErr    bool
		wantNTasks int
	}{
		"Success with no task file":                        {},
		"Success with empty task file":                     {taskFile: fileIsEmpty},
		"Success with task file containing a single task":  {taskFile: fileHasOneTask, wantNTasks: 1},
		"Success with task file containing multiple tasks": {taskFile: fileHasTwoTasks, wantNTasks: 2},

		// Error
		"Error when task file reads non-registered task type": {taskFile: fileHasNonRegisteredTask, wantErr: true},
		"Error when task file has bad syntax":                 {taskFile: fileHasBadSyntax, wantErr: true},
		"Error when task file is unreadable":                  {taskFile: fileIsDir, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			distro := &testDistro{name: wsltestutils.RandomDistroName(t)}

			distroDir := t.TempDir()
			taskFile := filepath.Join(distroDir, distro.Name()+".tasks")
			switch tc.taskFile {
			case fileNotExist:
			case fileIsEmpty:
				err := os.WriteFile(taskFile, []byte{}, 0600)
				require.NoError(t, err, "Setup: could not write empty task file")
			case fileHasOneTask:
				out := taskfileFromTemplate[emptyTask](t)
				err := os.WriteFile(taskFile, out, 0600)
				require.NoError(t, err, "Setup: could not write task file")
			case fileHasTwoTasks:
				out := taskfileFromTemplate[emptyTask](t)
				out = bytes.Repeat(out, 2)
				err := os.WriteFile(taskFile, out, 0600)
				require.NoError(t, err, "Setup: could not write task file")
			case fileHasNonRegisteredTask:
				out := taskfileFromTemplate[*testTask](t)
				err := os.WriteFile(taskFile, out, 0600)
				require.NoError(t, err, "Setup: could not write task file")
			case fileHasBadSyntax:
				err := os.WriteFile(taskFile, []byte("This\nis not valid\n\t\tYAML"), 0600)
				require.NoError(t, err, "Setup: could not write empty task file")
			case fileIsDir:
				err := os.MkdirAll(taskFile, 0600)
				require.NoError(t, err, "Setup: could not make a directory in task file's location")
			}

			// We pass a cancelled context so that no tasks are popped
			// and we can accurately assert on the task queue length.
			cancel()

			w, err := worker.New(ctx, distro, distroDir)
			if err == nil {
				defer w.Stop(ctx)
			}

			if tc.wantErr {
				require.Error(t, err, "worker.New should have returned an error")
				return
			}
			require.NoError(t, err, "worker.New should not return an error")
			require.NoError(t, w.CheckQueuedTaskCount(tc.wantNTasks), "Wrong number of queued tasks.")
		})
	}
}

func TestTaskProcessing(t *testing.T) {
	t.Parallel()

	type taskReturns int
	const (
		taskReturnsNil taskReturns = iota
		taskReturnsErr
		taskReturnsNeedsRetryErr
	)

	testCases := map[string]struct {
		unregisterAfterConstructor bool        // Triggers error in trying to get distro in LockAwake
		taskReturns                taskReturns // Causes the task to always return an error
		forceConnectionTimeout     bool        // Cancels the context while waiting for the GRPC connection to be established
		cancelTaskInProgress       bool        // Cancels as the task is running

		wantExecuteCalled bool
	}{
		"Success executing a task": {wantExecuteCalled: true},

		"Error when the distro is not registered":    {unregisterAfterConstructor: true},
		"Error when the connection times out":        {forceConnectionTimeout: true},
		"Error when a task in progress is cancelled": {cancelTaskInProgress: true, wantExecuteCalled: true},

		"Error when the task returns a generic error":   {taskReturns: taskReturnsErr, wantExecuteCalled: true},
		"Error when the task returns a NeedsRetryError": {taskReturns: taskReturnsNeedsRetryErr, wantExecuteCalled: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			d := &testDistro{
				name: wsltestutils.RandomDistroName(t),
			}

			w, err := worker.New(ctx, d, t.TempDir())
			require.NoError(t, err, "Setup: worker New() should return no error")
			defer w.Stop(ctx)

			// End of setup
			require.Nil(t, w.Connection(), "Connection() should return nil when there is no connection")

			if tc.unregisterAfterConstructor {
				d.Invalidate(ctx)
			}

			// Submit a task, wait for distro to wake up, and wait for slightly
			// more than the client waiting tickrate
			const distroWakeUpTime = 1 * time.Second
			const clientTickPeriod = 1200 * time.Millisecond

			ttask := &testTask{}
			switch tc.taskReturns {
			case taskReturnsErr:
				ttask.Returns = errors.New("testTask error")
			case taskReturnsNeedsRetryErr:
				ttask.Returns = task.NeedsRetryError{SourceErr: errors.New("testTask error")}
			}

			if tc.cancelTaskInProgress {
				// This particular task will always retry in a loop
				// Long delay to ensure we can reliably cancell it in progress
				ttask.Delay = 10 * time.Second
				ttask.Returns = errors.New("testTask error: this error should never be triggered")
			}

			err = w.SubmitTasks(ttask)
			require.NoError(t, err, "SubmitTask() should work without returning any errors")

			// Ensuring the distro is awakened (if registered) after task submission
			wantState := "Running"
			if tc.unregisterAfterConstructor {
				wantState = "Unregistered"
			}

			require.Eventuallyf(t, func() bool { return d.state() == wantState }, 5*distroWakeUpTime, 500*time.Millisecond,
				"distro should have been %q after SubmitTask(). Current state is %q", wantState, d.state())

			// Testing task before an active connection is established
			// We sleep to ensure at least one tick has gone by in the "wait for connection"
			time.Sleep(clientTickPeriod)
			require.Nil(t, w.Connection(), "Connection should return nil when there is no connection")
			require.Equal(t, int32(0), ttask.ExecuteCalls.Load(), "Task unexpectedly executed without a connection")

			if tc.forceConnectionTimeout {
				cancel() // Simulates a timeout
				time.Sleep(clientTickPeriod)
			}

			// Testing task with active connection
			w.SetConnection(&mockConnection{})

			if !tc.wantExecuteCalled {
				time.Sleep(2 * clientTickPeriod)
				require.Equal(t, int32(0), ttask.ExecuteCalls.Load(), "Task executed unexpectedly")
				return
			}

			require.Eventuallyf(t, func() bool { return w.Connection() != nil }, clientTickPeriod, 100*time.Millisecond,
				"Client should become non-nil after setting the connection")

			// Wait for task to start
			require.Eventuallyf(t, func() bool { return ttask.ExecuteCalls.Load() == 1 }, 2*clientTickPeriod, 100*time.Millisecond,
				"Task was executed fewer times than expected. Expected 1 and executed %d.", ttask.ExecuteCalls.Load())

			if tc.cancelTaskInProgress {
				// Cancelling and waiting for cancellation to propagate, then ensure it did so.
				cancel()
				require.Eventually(t, func() bool { return ttask.WasCancelled.Load() }, 100*time.Millisecond, time.Millisecond,
					"Task should be cancelled when the task processing context is cancelled")

				// Giving some time to ensure retry is never attempted.
				time.Sleep(100 * time.Millisecond)
				require.Equal(t, int32(1), ttask.ExecuteCalls.Load(), "Task should never be retried")
				return
			}

			time.Sleep(time.Second)
			require.Equal(t, int32(1), ttask.ExecuteCalls.Load(), "Task should not execute more than once")

			switch tc.taskReturns {
			case taskReturnsNil, taskReturnsErr:
				require.NoError(t, w.CheckQueuedTaskCount(0), "No tasks should remain in the queue")
				require.NoError(t, w.CheckTotalTaskCount(0), "No tasks should remain in storage")
			case taskReturnsNeedsRetryErr:
				require.NoError(t, w.CheckQueuedTaskCount(0), "No tasks should remain in the queue")
				require.NoError(t, w.CheckTotalTaskCount(1), "The task that failed with NeedsRetryError should be in storage")
			}
		})
	}
}

func TestSubmitTaskFailsCannotWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	distro := &testDistro{name: wsltestutils.RandomDistroName(t)}
	distroDir := t.TempDir()
	taskFile := filepath.Join(distroDir, distro.Name()+".tasks")

	w, err := worker.New(ctx, distro, distroDir)
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	err = os.RemoveAll(taskFile)
	require.NoError(t, err, "Could not remove distro task backup file")

	err = os.MkdirAll(taskFile, 0600)
	require.NoError(t, err, "Could not make dir at distro task file's location")

	err = w.SubmitTasks(&emptyTask{})
	require.Error(t, err, "Submitting a task when the task file is not writable should cause an error")
}

func TestSetConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := &testDistro{
		name: wsltestutils.RandomDistroName(t),
	}

	w, err := worker.New(ctx, d, t.TempDir())
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	conn1 := &mockConnection{}
	conn2 := &mockConnection{}

	require.Nil(t, w.Connection(), "Client() should return nil because the connection has not been set yet")
	require.False(t, w.IsActive(), "IsActive() should return false because the connection has not been set yet")

	// Set first connection as active
	w.SetConnection(conn1)

	require.True(t, w.IsActive(), "IsActive() should return true because the connection has been set")

	// GetClient twice and ensure we ping the same service
	const conn1calls = 2
	for i := range conn1calls {
		c := w.Connection()
		require.NotNil(t, c, "client should be non-nil after setting a connection")
		err = c.SendProAttachment("123")
		require.NoError(t, err, "SendProAttachment attempt #%d should have been done successfully", i)
		require.EqualValues(t, i+1, conn1.proAttachmentCount.Load(), "second server should be pinged after c.Ping (iteration #%d)", i)
	}

	require.Zero(t, conn2.proAttachmentCount.Load(), "second connection should not be used yet")

	// Set second connection as active
	w.SetConnection(conn2)
	require.True(t, w.IsActive(), "IsActive() should return true even if the connection has changed")

	// Ping on renewed connection (new wsl instance service) and ensure only the second service receives the pings
	c := w.Connection()
	require.NotNil(t, c, "client should be non-nil after setting a connection")
	err = c.SendProAttachment("123")
	require.NoError(t, err, "SendProAttachment should have been done successfully")
	require.EqualValues(t, 1, conn2.proAttachmentCount.Load(), "second connection's ProAttach should have been called")

	require.EqualValues(t, conn1calls, conn1.proAttachmentCount.Load(), "first service should not have been called again")

	// Set connection to nil and ensure that no pings are made
	w.SetConnection(nil)
	require.Nil(t, w.Connection(), "Client() should return a nil because the connection has been set to nil")
	require.False(t, w.IsActive(), "IsActive() should return false because the connection has been set to nil")

	require.EqualValues(t, conn1calls, conn1.proAttachmentCount.Load(), "first connection should not have been used setting the connection to nil")
	require.EqualValues(t, 1, conn2.proAttachmentCount.Load(), "second connection should not have been used setting the connection to nil")
}

func TestSetConnectionOnClosedConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := &testDistro{
		name: wsltestutils.RandomDistroName(t),
	}

	w, err := worker.New(ctx, d, t.TempDir())
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	conn1 := &mockConnection{}
	conn2 := &mockConnection{}

	w.SetConnection(conn1)
	conn1.Close()

	w.SetConnection(conn2)

	// New connection is functional.
	err = w.Connection().SendLandscapeConfig("123")
	require.NoError(t, err, "SendLandscapeConfig should have been done successfully")
	require.EqualValues(t, 1, conn2.LandscapeConfigCount.Load(), "second service have been used once")
}

func TestTaskDeferral(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakReload bool
		breakSubmit bool

		wantSubmitErr bool
		wantReloadErr bool
	}{
		"Success reloading two tasks": {},

		"Error if the task file cannot be read":    {breakReload: true, wantReloadErr: true},
		"Error if the task file cannot be written": {breakSubmit: true, wantSubmitErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			d := &testDistro{
				name: wsltestutils.RandomDistroName(t),
			}

			storage := t.TempDir()
			taskFile := filepath.Join(storage, d.Name()+".tasks")

			w, err := worker.New(ctx, d, storage)
			require.NoError(t, err, "Setup: unexpected error creating the worker")
			defer w.Stop(ctx)

			queuedTask := emptyTask{ID: uuid.NewString()}
			deferredTask := emptyTask{ID: uuid.NewString()}

			// Testing task with active connection
			w.SetConnection(&mockConnection{})

			// blocker is a task meant to block task processing
			blocker := newBlockingTask(ctx)
			defer blocker.complete()

			err = w.SubmitTasks(blocker)
			require.NoError(t, err, "SubmitTasks should have succeeded for a queued task")

			err = w.SubmitTasks(queuedTask)
			require.NoError(t, err, "SubmitTasks should have succeeded for a second queued task")

			if tc.breakSubmit {
				// We wait until the blocking task is popped to avoid a filesystem race:
				// write:           TaskManager.NextTask
				// delete+write:    testutils.ReplaceFileWithDir
				require.Eventually(t, func() bool {
					return w.CheckTotalTaskCount(1) == nil
				}, 5*time.Second, 500*time.Millisecond, "Setup: Blocking task was never popped from queue")

				testutils.ReplaceFileWithDir(t, taskFile, "Setup: could not replace task file with dir to interfere with SubmitDeferredTasks")
			}

			err = w.SubmitDeferredTasks(deferredTask)
			if tc.wantSubmitErr {
				require.Error(t, err, "SubmitDeferredTasks should have returned an error")
				return
			}
			require.NoError(t, err, "SubmitDeferredTasks should have succeeded for a deferred task")

			// Wait until blocking task is popped from the queue
			require.Eventually(t, blocker.executing.Load, 10*time.Second, 100*time.Millisecond, "Number of queued tasks never became 1")

			// One task is queued and the other one is deferred
			require.NoError(t, w.CheckQueuedTaskCount(1), "Expected only one task queued behind the blocker")
			require.NoError(t, w.CheckTotalTaskCount(2), "Expected two tasks stored after the blocker is popped")

			w.EnqueueDeferredTasks()

			require.NoError(t, w.CheckQueuedTaskCount(2), "Tasks did not reload into the queue as expected")
			require.NoError(t, w.CheckTotalTaskCount(2), "Tasks did not reload into the list as expected")

			blocker.complete()

			requireEventuallyTaskCompletes(t, queuedTask, "Queued task should have been completed")
			requireEventuallyTaskCompletes(t, deferredTask, "Deferred task should have been completed")

			require.NoError(t, w.CheckQueuedTaskCount(0), "Completed tasks should have been removed from the queue")
			require.NoError(t, w.CheckTotalTaskCount(0), "Completed tasks should have been removed from storage")

			// Submit a task without a blocker
			// This tests the queue refreshment
			newTask := emptyTask{ID: uuid.NewString()}
			err = w.SubmitDeferredTasks(newTask)

			require.NoError(t, err, "Submitting a deferred task should cause no errors")
			require.NoError(t, w.CheckQueuedTaskCount(0), "Task was queued unexpectedly")
			require.NoError(t, w.CheckTotalTaskCount(1), "Task was not stored as expected")

			w.EnqueueDeferredTasks()
			requireEventuallyTaskCompletes(t, newTask, "Deferred task should have been completed")
		})
	}
}

func TestTaskDeduplication(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		taskWithIs bool
	}{
		"Success with plain task":              {},
		"Success with task with overloaded Is": {taskWithIs: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			d := &testDistro{
				name: wsltestutils.RandomDistroName(t),
			}

			storage := t.TempDir()

			w, err := worker.New(ctx, d, storage)
			require.NoError(t, err, "Setup: unexpected error creating the worker")
			defer w.Stop(ctx)

			w.SetConnection(&mockConnection{})

			// These are equivalent, they should be de-duplicated
			blocker := newBlockingTask(ctx)

			var task1 task.Task = emptyTask{ID: "123"}
			var taskEq task.Task = emptyTask{ID: "123"}

			if tc.taskWithIs {
				// Different delays to ensure it is the "Is" that is making the comparison
				task1 = &testTask{ID: "ABC", Delay: time.Second}
				taskEq = &testTask{ID: "ABC", Delay: 5 * time.Second}
			}

			err = w.SubmitTasks(blocker)
			require.NoError(t, err, "SubmitTasks should return no error")
			require.Eventually(t, blocker.executing.Load, 5*time.Second, 500*time.Millisecond, "Blocker task was never dequeued")

			// Unique task: normal submission
			err = w.SubmitTasks(task1)
			require.NoError(t, err, "SubmitTasks should return no error")
			require.NoError(t, w.CheckQueuedTaskCount(1), "Submitting a task should add it to the queue")
			require.NoError(t, w.CheckTotalTaskCount(1), "Submitting a task should increase the total task count by one")

			// Unique task: normal submission
			err = w.SubmitTasks(emptyTask{ID: "hello!"})
			require.NoError(t, err, "SubmitTasks should return no error")
			require.NoError(t, w.CheckQueuedTaskCount(2), "Submitting a second task should add it to the queue")
			require.NoError(t, w.CheckTotalTaskCount(2), "Submitting a second task should increase the total task count by one")

			// Check that re-submitting a task removes the old one
			err = w.SubmitTasks(taskEq)
			require.NoError(t, err, "SubmitTasks should return no error")
			require.NoError(t, w.CheckQueuedTaskCount(2), "Submitting a repeated task should not change the queue size")
			require.NoError(t, w.CheckTotalTaskCount(2), "Submitting a repeated task should not change the task count")

			// Check that re-submitting a task as deferred removes the old one
			err = w.SubmitDeferredTasks(taskEq)
			require.NoError(t, err, "SubmitDeferredTasks should return no error")
			require.NoError(t, w.CheckQueuedTaskCount(1), "Submitting a repeated deferred task should decrease the queue size by one")
			require.NoError(t, w.CheckTotalTaskCount(2), "Submitting a repeated deferred task should not change the total task count")

			// Check that re-submitting a deferred task removes the old one
			// This caused https://warthogs.atlassian.net/browse/UDENG-1848
			err = w.SubmitTasks(taskEq)
			require.NoError(t, err, "SubmitTasks should return no error")
			require.NoError(t, w.CheckQueuedTaskCount(2), "Submitting a task that was already deferred should increase the queue size by one")
			require.NoError(t, w.CheckTotalTaskCount(2), "Submitting a task that was already deferred should not change the total task count")
		})
	}
}

func TestFailedTaskIsDeferred(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testDistro{
		name: wsltestutils.RandomDistroName(t),
	}

	storage := t.TempDir()

	w, err := worker.New(ctx, d, storage)
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	w.SetConnection(&mockConnection{})

	// Submit the failing task
	failingTask := testTask{Returns: task.NeedsRetryError{SourceErr: errors.New("mock error")}}
	err = w.SubmitTasks(&failingTask)
	require.NoError(t, err, "SubmitTasks should return no error")

	require.Eventually(t, func() bool {
		return failingTask.ExecuteCalls.Load() == 1
	}, 5*time.Second, 100*time.Millisecond, "Task should have started executing")
	require.NoError(t, w.CheckQueuedTaskCount(0), "Task should have been popped from the queue")

	require.Eventually(t, func() bool {
		return w.CheckTotalTaskCount(1) == nil
	}, 5*time.Second, 100*time.Millisecond, "Failing task should have been re-submitted after failure")
	require.NoError(t, w.CheckQueuedTaskCount(0), "Task should not have been submitted into the queue, but rather deferred")
}

func requireEventuallyTaskCompletes(t *testing.T, task emptyTask, msg string, args ...any) {
	t.Helper()

	require.Eventually(t, func() bool {
		return completedEmptyTasks.Has(task.ID)
	}, 5*time.Second, 100*time.Millisecond, msg, args)
}

// completedEmptyTasks tracks which empty tasks have completed. We need to use this global
// variable because tasks may be written to file and read back, so no callbacks or pointers
// can be used.
var completedEmptyTasks = testutils.NewSet[string]()

type emptyTask struct {
	ID string
}

func (t emptyTask) Execute(ctx context.Context, _ task.Connection) error {
	completedEmptyTasks.Set(t.ID)
	return nil
}

func (t emptyTask) String() string {
	return "Empty test task"
}

type testTask struct {
	// ExecuteCalls counts the number of times Execute is called
	ExecuteCalls atomic.Int32

	// Delay simulates a processing time for the task
	Delay time.Duration

	// Returns is the value that Execute will return
	Returns error

	// WasCancelled is true if the task Execute context is Done
	WasCancelled atomic.Bool

	ID string
}

// MarshalYAML is necessary to avoid races between Execute and Save.
func (t *testTask) MarshalYAML() (interface{}, error) {
	return struct {
		ID      string
		Delay   time.Duration
		Returns error
	}{
		ID:      t.ID,
		Delay:   t.Delay,
		Returns: t.Returns,
	}, nil
}

func (t *testTask) Execute(ctx context.Context, _ task.Connection) error {
	t.ExecuteCalls.Add(1)
	select {
	case <-time.After(t.Delay):
		return t.Returns
	case <-ctx.Done():
		t.WasCancelled.Store(true)
		return ctx.Err()
	}
}

func (t *testTask) String() string {
	return "Test task"
}

func (t *testTask) Is(other task.Task) bool {
	o, ok := other.(*testTask)
	if !ok {
		return false
	}
	return t.ID == o.ID
}

// blockingTask is a task that blocks execution until complete() is called.
type blockingTask struct {
	ctx       context.Context
	complete  func()
	executing atomic.Bool `yaml:"-"`
}

func newBlockingTask(ctx context.Context) *blockingTask {
	ctx, cancel := context.WithCancel(ctx)
	return &blockingTask{
		ctx:      ctx,
		complete: cancel,
	}
}

// MarshalYAML is necessary to avoid races between Execute and Save.
func (t *blockingTask) MarshalYAML() (interface{}, error) {
	return struct{}{}, nil
}

func (t *blockingTask) Execute(ctx context.Context, _ task.Connection) error {
	t.executing.Store(true)
	defer t.executing.Store(false)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return nil
	}
}

func (t *blockingTask) String() string {
	return "Blocking task"
}

type testDistro struct {
	// Change these freely to modify test behaviour
	name    string      // The name of the distro
	invalid atomic.Bool // Whether the distro is valid or not

	// TODO: Is this used?
	LockAwakeError error // LockAwake will throw this error (unless it is nil)

	// Do not use directly
	runningRefCount int
	runningMu       sync.RWMutex
}

// state returns the state of the distro as specified by wsl.exe. Possible states:
// - Installing
// - Running
// - Stopped
// - Unregistered.
func (d *testDistro) state() string {
	if d.invalid.Load() {
		return "Unregistered"
	}

	d.runningMu.RLock()
	defer d.runningMu.RUnlock()

	if d.runningRefCount != 0 {
		return "Running"
	}

	return "Stopped"
}

func (d *testDistro) Name() string {
	return d.name
}

func (d *testDistro) LockAwake() error {
	if err := d.LockAwakeError; err != nil {
		return err
	}

	if !d.IsValid() {
		return fmt.Errorf("LockAwake: testDistro %q is not valid", d.name)
	}

	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	d.runningRefCount++
	return nil
}

func (d *testDistro) ReleaseAwake() error {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	if d.runningRefCount == 0 {
		return errors.New("excess calls to ReleaseAwake")
	}

	d.runningRefCount--

	return nil
}

func (d *testDistro) IsValid() bool {
	return !d.invalid.Load()
}

func (d *testDistro) Invalidate(ctx context.Context) {
	d.invalid.Store(true)
}

func taskfileFromTemplate[T task.Task](t *testing.T) []byte {
	t.Helper()

	in, err := os.ReadFile(filepath.Join(testutils.TestFamilyPath(t), "template.tasks"))
	require.NoError(t, err, "Setup: could not read tasks template")

	tmpl := template.Must(template.New(t.Name()).Parse(string(in)))

	w := &bytes.Buffer{}

	taskType := reflect.TypeOf((*T)(nil)).Elem().String()
	err = tmpl.Execute(w, taskType)
	require.NoError(t, err, "Setup: could not execute template task file")

	return w.Bytes()
}

type mockConnection struct {
	proAttachmentCount   atomic.Int32
	LandscapeConfigCount atomic.Int32
	closed               atomic.Bool
}

func (conn *mockConnection) SendProAttachment(proToken string) error {
	conn.proAttachmentCount.Add(1)
	return nil
}

func (conn *mockConnection) SendLandscapeConfig(lpeConfig string) error {
	conn.LandscapeConfigCount.Add(1)
	return nil
}

func (conn *mockConnection) Close() {
	conn.closed.Store(true)
}
