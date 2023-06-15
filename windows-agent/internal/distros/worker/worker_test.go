package worker_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common/golden"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/worker"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
		fileHasTooManyTasks

		fileHasBadSyntax
		fileHasNonRegisteredTask
		fileIsDir
	)

	testCases := map[string]struct {
		taskFile    taskFileState
		fillUpQueue bool

		withProvisioning     bool
		emptyProvisioning    bool
		provisioningTasksErr bool

		wantErr    bool
		wantNTasks int
	}{
		"Success with no task file":                        {},
		"Success with empty task file":                     {taskFile: fileIsEmpty},
		"Success with task file containing a single task":  {taskFile: fileHasOneTask, wantNTasks: 1},
		"Success with task file containing too many tasks": {taskFile: fileHasTooManyTasks, wantNTasks: worker.TaskQueueSize},

		"Success with empty provisioning":       {withProvisioning: true, emptyProvisioning: true},
		"Success with single-task provisioning": {withProvisioning: true, wantNTasks: 1},

		// Error
		"Error when task file reads non-registered task type": {taskFile: fileHasNonRegisteredTask, wantErr: true},
		"Error when task file has bad syntax":                 {taskFile: fileHasBadSyntax, wantErr: true},
		"Error when task file is unreadable":                  {taskFile: fileIsDir, wantErr: true},
		"Error when ProvisioningTasks fails":                  {withProvisioning: true, provisioningTasksErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			distro := &testDistro{name: testutils.RandomDistroName(t)}

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
			case fileHasTooManyTasks:
				out := taskfileFromTemplate[emptyTask](t)
				out = bytes.Repeat(out, worker.TaskQueueSize+5)
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

			var args []worker.Option
			if tc.withProvisioning {
				c := &mockProvisioning{
					provisioningTasksErr:        tc.provisioningTasksErr,
					privisioningTasksReturnsNil: tc.emptyProvisioning,
				}
				args = append(args, worker.WithProvisioning(c))
			}

			// We pass a cancelled context so that no tasks are popped
			// and we can accurately assert on the task queue length.
			cancel()

			w, err := worker.New(ctx, distro, distroDir, args...)
			if err == nil {
				defer w.Stop(ctx)
			}

			if tc.wantErr {
				require.Error(t, err, "worker.New should have returned an error")
				return
			}
			require.NoError(t, err, "worker.New should not return an error")
			require.NoError(t, w.CheckQueuedTasks(tc.wantNTasks), "Wrong number of queued tasks.")
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			d := &testDistro{
				name: testutils.RandomDistroName(t),
			}

			w, err := worker.New(ctx, d, t.TempDir())
			require.NoError(t, err, "Setup: worker New() should return no error")
			defer w.Stop(ctx)

			wslInstanceService := newTestService(t)
			conn := wslInstanceService.newClientConnection(t)

			// End of setup
			require.Equal(t, nil, w.Client(), "Client() should return nil when there is no connection")

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

			require.Eventuallyf(t, func() bool { return d.state() == wantState }, distroWakeUpTime, 200*time.Millisecond,
				"distro should have been %q after SubmitTask(). Current state is %q", wantState, d.state())

			// Testing task before an active connection is established
			// We sleep to ensure at least one tick has gone by in the "wait for connection"
			time.Sleep(clientTickPeriod)
			require.Equal(t, nil, w.Client(), "Client should return nil when there is no connection")
			require.Equal(t, int32(0), ttask.ExecuteCalls.Load(), "Task unexpectedly executed without a connection")

			if tc.forceConnectionTimeout {
				cancel() // Simulates a timeout
				time.Sleep(clientTickPeriod)
			}

			// Testing task with with active connection
			w.SetConnection(conn)

			if !tc.wantExecuteCalled {
				time.Sleep(2 * clientTickPeriod)
				require.Equal(t, int32(0), ttask.ExecuteCalls.Load(), "Task executed unexpectedly")
				return
			}

			require.Eventuallyf(t, func() bool { return w.Client() != nil }, clientTickPeriod, 100*time.Millisecond,
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
			case taskReturnsNil:
				fallthrough
			case taskReturnsErr:
				require.NoError(t, w.CheckQueuedTasks(0), "No tasks should remain in the queue")
				require.NoError(t, w.CheckStoredTasks(0), "No tasks should remain in storage")
			case taskReturnsNeedsRetryErr:
				require.NoError(t, w.CheckQueuedTasks(0), "No tasks should remain in the queue")
				require.NoError(t, w.CheckStoredTasks(1), "The task that failed with NeedsRetryError should be in storage")
			}
		})
	}
}

func TestSubmitTaskFailsCannotWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	distro := &testDistro{name: testutils.RandomDistroName(t)}
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

func TestSubmitTaskFailsWithFullQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := &testDistro{
		name: testutils.RandomDistroName(t),
	}

	w, err := worker.New(ctx, d, t.TempDir())
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	// We submit a first task that will be dequeued and block task processing until
	// there is a connection (i.e. forever) or until it times out after a minute.
	err = w.SubmitTasks(&testTask{})
	require.NoErrorf(t, err, "SubmitTask() should not fail when the distro is active and the queue is empty.\nSubmitted: %d.\nMax: %d", 1, worker.TaskQueueSize)

	// We fill up the queue
	for i := 0; i < worker.TaskQueueSize; i++ {
		err := w.SubmitTasks(&testTask{})
		require.NoErrorf(t, err, "SubmitTask() should not fail when the distro is active and the queue is not full.\nSubmitted: %d.\nMax: %d", i+1, worker.TaskQueueSize)
	}

	// We ensure that adding one more task will return an error
	err = w.SubmitTasks(&testTask{})
	require.Errorf(t, err, "SubmitTask() should fail when the queue is full")
}

func TestSetConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := &testDistro{
		name: testutils.RandomDistroName(t),
	}

	w, err := worker.New(ctx, d, t.TempDir())
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	wslInstanceService1 := newTestService(t)
	conn1 := wslInstanceService1.newClientConnection(t)

	wslInstanceService2 := newTestService(t)
	conn2 := wslInstanceService2.newClientConnection(t)

	require.Equal(t, nil, w.Client(), "Client() should return nil because the connection has not been set yet")
	require.False(t, w.IsActive(), "IsActive() should return false because the connection has not been set yet")

	// Set first connection as active
	w.SetConnection(conn1)

	require.True(t, w.IsActive(), "IsActive() should return true because the connection has been set")

	// GetClient twice and ensure we ping the same service
	const service1pings = 2
	for i := 0; i < service1pings; i++ {
		c := w.Client()
		require.NotEqual(t, nil, c, "client should be non-nil after setting a connection")
		_, err = c.Ping(ctx, &wslserviceapi.Empty{})
		require.NoError(t, err, "Ping attempt #%d should have been done successfully", i)
		require.Equal(t, i+1, wslInstanceService1.pingCount, "second server should be pinged after c.Ping (iteration #%d)", i)
	}

	require.Equal(t, 0, wslInstanceService2.pingCount, "second service should not be called yet")

	// Set second connection as active
	w.SetConnection(conn2)
	require.True(t, w.IsActive(), "IsActive() should return true even if the connection has changed")

	// Ping on renewed connection (new wsl instance service) and ensure only the second service receives the pings
	c := w.Client()
	require.NotEqual(t, nil, c, "client should be non-nil after setting a connection")
	_, err = c.Ping(ctx, &wslserviceapi.Empty{})
	require.NoError(t, err, "Ping should have been done successfully")
	require.Equal(t, 1, wslInstanceService2.pingCount, "second server should be pinged after c.Ping")

	require.Equal(t, service1pings, wslInstanceService1.pingCount, "first service should not have received pings after setting the connection to the second service")

	// Set connection to nil and ensure that no pings are made
	w.SetConnection(nil)
	require.Equal(t, nil, w.Client(), "Client() should return a nil because the connection has been set to nil")
	require.False(t, w.IsActive(), "IsActive() should return false because the connection has been set to nil")

	require.Equal(t, service1pings, wslInstanceService1.pingCount, "first service should not have received pings after setting the connection to nil")
	require.Equal(t, 1, wslInstanceService2.pingCount, "second service should not have received pings after setting the connection to nil")
}

func TestSetConnectionOnClosedConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := &testDistro{
		name: testutils.RandomDistroName(t),
	}

	w, err := worker.New(ctx, d, t.TempDir())
	require.NoError(t, err, "Setup: unexpected error creating the worker")
	defer w.Stop(ctx)

	wslInstanceService1 := newTestService(t)
	conn1 := wslInstanceService1.newClientConnection(t)

	wslInstanceService2 := newTestService(t)
	conn2 := wslInstanceService2.newClientConnection(t)

	w.SetConnection(conn1)
	_ = conn1.Close()

	w.SetConnection(conn2)

	// New connection is functional.
	_, err = w.Client().Ping(ctx, &wslserviceapi.Empty{})
	require.NoError(t, err, "Ping should have been done successfully")
	require.Equal(t, 1, wslInstanceService2.pingCount, "second service should be called once")
}

type testService struct {
	wslserviceapi.UnimplementedWSLServer
	pingCount int
	port      uint16
}

func (s *testService) Ping(context.Context, *wslserviceapi.Empty) (*wslserviceapi.Empty, error) {
	s.pingCount++
	return &wslserviceapi.Empty{}, nil
}

// newTestService creates a testService and starts serving asyncronously.
func newTestService(t *testing.T) *testService {
	t.Helper()

	server := grpc.NewServer()

	lis, err := net.Listen("tcp4", "localhost:")
	require.NoErrorf(t, err, "Setup: could not listen.")

	fields := strings.Split(lis.Addr().String(), ":")
	portTmp, err := strconv.ParseUint(fields[len(fields)-1], 10, 16)
	require.NoError(t, err, "Setup: could not parse address")

	service := testService{port: uint16(portTmp)}
	wslserviceapi.RegisterWSLServer(server, &service)
	go func() {
		err := server.Serve(lis)
		if err != nil {
			t.Logf("Setup: server.Serve returned non-nil error: %v", err)
		}
	}()

	t.Cleanup(server.Stop)

	t.Logf("Setup: Started listening at %q", lis.Addr())

	return &service
}

func (s testService) newClientConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", s.port)

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctxTimeout, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	require.NoError(t, err, "Setup: could not contact the grpc server at %q", addr)

	t.Cleanup(func() { conn.Close() })

	return conn
}

type emptyTask struct{}

func (t emptyTask) Execute(ctx context.Context, _ wslserviceapi.WSLClient) error {
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
}

func (t *testTask) Execute(ctx context.Context, _ wslserviceapi.WSLClient) error {
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

	in, err := os.ReadFile(filepath.Join(golden.TestFamilyPath(t), "template.tasks"))
	require.NoError(t, err, "Setup: could not read tasks template")

	tmpl := template.Must(template.New(t.Name()).Parse(string(in)))

	w := &bytes.Buffer{}

	taskType := reflect.TypeOf((*T)(nil)).Elem().String()
	err = tmpl.Execute(w, taskType)
	require.NoError(t, err, "Setup: could not execute template task file")

	return w.Bytes()
}

type mockProvisioning struct {
	provisioningTasksErr        bool
	privisioningTasksReturnsNil bool
}

func (c mockProvisioning) ProvisioningTasks(ctx context.Context) ([]task.Task, error) {
	if c.provisioningTasksErr {
		return nil, errors.New("mock error")
	}
	if c.privisioningTasksReturnsNil {
		return nil, nil
	}
	return []task.Task{&testTask{}}, nil
}
