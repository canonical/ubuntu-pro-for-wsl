// Package worker manages the execution and queue of tasks.
package worker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/ubuntu/decorate"
)

type distro interface {
	Name() string

	LockAwake() error
	ReleaseAwake() error

	IsValid() bool
	Invalidate(context.Context)
}

// Connection encapsulates the logic behind sending and receiving messages
// with the WSL-Pro-Service.
type Connection interface {
	SendProAttachment(proToken string) error
	SendLandscapeConfig(lpeConfig, hostagentUID string) error
	Close()
}

// Worker contains all the logic around task queueing and execution for one particular distro.
type Worker struct {
	distro  distro
	manager *taskManager

	cancel     context.CancelFunc
	processing chan struct{}

	conn   Connection
	connMu sync.RWMutex
}

// New creates a new worker and starts it. Call Stop when you're done to avoid leaking the task execution goroutine.
func New(ctx context.Context, d distro, storageDir string) (w *Worker, err error) {
	defer decorate.OnError(&err, "distro %q: could not create worker", d.Name())

	storagePath := filepath.Join(storageDir, d.Name()+".tasks")

	tm, err := newTaskManager(storagePath)
	if err != nil {
		return nil, err
	}

	w = &Worker{
		distro:  d,
		manager: tm,
	}

	w.start(ctx)

	return w, nil
}

// IsActive returns true when the worker is running, and there exists an active
// connection to its GRPC service.
func (w *Worker) IsActive() bool {
	w.connMu.RLock()
	defer w.connMu.RUnlock()

	return w.conn != nil
}

// Connection returns the client to the WSL task service.
// Connection returns nil when no connection is set up.
func (w *Worker) Connection() Connection {
	w.connMu.RLock()
	defer w.connMu.RUnlock()

	return w.conn
}

// SetConnection removes the connection associated with the distro.
func (w *Worker) SetConnection(conn Connection) {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	if w.conn != nil {
		w.conn.Close()
	}

	w.conn = conn
}

// start starts the main task processing goroutine.
func (w *Worker) start(ctx context.Context) {
	log.Debugf(ctx, "Distro %q: starting task processing", w.distro.Name())

	ctx, cancel := context.WithCancel(ctx)
	w.processing = make(chan struct{})
	go w.processTasks(ctx)
	w.cancel = cancel
}

// Stop stops the main task processing goroutine and wait for it to be done.
func (w *Worker) Stop(ctx context.Context) {
	log.Debugf(ctx, "Distro %q: stopping task processing", w.distro.Name())
	w.cancel()
	<-w.processing
	w.SetConnection(nil)
}

// SubmitTasks enqueues one or more task on our current worker list. The task will wake up
// the distro and be performed as soon as it reaches the beginning of the queue.
//
// It will return an error if the distro has been cleaned up or the task queue is full.
func (w *Worker) SubmitTasks(tasks ...task.Task) (err error) {
	defer decorate.OnError(&err, "distro %q: tasks %q: could not submit", w.distro.Name(), tasks)

	if len(tasks) == 0 {
		return nil
	}

	log.Infof(context.TODO(), "Distro %q: Submitting tasks %q to queue", w.distro.Name(), tasks)
	return w.manager.Submit(false, tasks...)
}

// SubmitDeferredTasks takes one or more tasks into our current worker list.
//
// The task(s) won't wake up the distro, instead wait until it is awake. This does
// NOT necessarily mean it'll run after non-deferred tasks.
//
// It will return an error if the distro has been cleaned up.
func (w *Worker) SubmitDeferredTasks(tasks ...task.Task) (err error) {
	defer decorate.OnError(&err, "distro %q: tasks %q: could not submit", w.distro.Name(), tasks)

	if len(tasks) == 0 {
		return nil
	}

	log.Infof(context.TODO(), "Distro %q: Submitting tasks %q to queue", w.distro.Name(), tasks)

	return w.manager.Submit(true, tasks...)
}

// EnqueueDeferredTasks takes all deferred tasks and promotes them
// to regular tasks.
func (w *Worker) EnqueueDeferredTasks() {
	w.manager.EnqueueDeferredTasks()
}

// processTasks is the main loop for the distro, processing any existing tasks while starting and releasing
// locks to distro,.
func (w *Worker) processTasks(ctx context.Context) {
	defer close(w.processing)

	for {
		t, ok := w.manager.NextTask(ctx)
		if !ok {
			return
		}

		resultErr := w.processSingleTask(ctx, t)

		var target unreachableDistroError
		if errors.As(resultErr, &target) {
			log.Errorf(ctx, "Distro %q: task %q: distro not reachable: %v", w.distro.Name(), t, target.sourceErr)
			w.distro.Invalidate(ctx)
			continue
		}

		err := w.manager.TaskDone(ctx, t, resultErr)
		if err != nil {
			log.Errorf(ctx, "Distro %q: %v", w.distro.Name(), err)
		}
	}
}

type unreachableDistroError struct {
	sourceErr error
}

func newUnreachableDistroErr(err error) error {
	if err == nil {
		return nil
	}
	return unreachableDistroError{
		sourceErr: err,
	}
}

func (err unreachableDistroError) Error() string {
	return fmt.Sprintf("distro cannot be reached: %v", err.sourceErr)
}

func (w *Worker) processSingleTask(ctx context.Context, t task.Task) error {
	log.Debugf(ctx, "Distro %q: starting task %q", w.distro.Name(), t)

	if !w.distro.IsValid() {
		return newUnreachableDistroErr(errors.New("distro marked as invalid"))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := w.distro.LockAwake(); err != nil {
		return newUnreachableDistroErr(err)
	}
	//nolint:errcheck // Nothing we can do about it
	defer w.distro.ReleaseAwake()

	log.Debugf(ctx, "Distro %q: distro is running.", w.distro.Name())

	client, err := w.waitForActiveConnection(ctx)
	if err != nil {
		return fmt.Errorf("task %v: could not start task: %w", t, err)
	}

	if err := t.Execute(ctx, client); err != nil {
		return fmt.Errorf("distro %q: task %q failed: %w", w.distro.Name(), t, err)
	}

	log.Debugf(ctx, "Distro %q: task %q: task completed successfully", w.distro.Name(), t)
	return nil
}

func (w *Worker) waitForActiveConnection(ctx context.Context) (conn Connection, err error) {
	log.Debugf(ctx, "Distro %q: ensuring active connection.", w.distro.Name())

	for i := 0; i < 5; i++ {
		conn, err = func() (Connection, error) {
			// Potentially restart distro if it was stopped for some reason
			if err := w.distro.LockAwake(); err != nil {
				return nil, newUnreachableDistroErr(err)
			}
			//nolint:errcheck // Nothing we can do about it
			defer w.distro.ReleaseAwake()

			// Connect to GRPC client
			client, err := w.waitForClient(ctx)
			if err != nil {
				return nil, err
			}

			log.Debugf(ctx, "Distro %q: connection is active.", w.distro.Name())
			return client, nil
		}()

		if err == nil {
			break
		}
	}

	return conn, err
}

// waitForClient waits for a valid GRPC client to connect to. It will retry for a while before
// erroring out.
func (w *Worker) waitForClient(ctx context.Context) (Connection, error) {
	timedOutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tickRate := 0 * time.Second
	for {
		select {
		case <-ctx.Done():
			// Context cancelled means agent teardown.
			return nil, fmt.Errorf("stopped waiting for client: %v", timedOutCtx.Err())
		case <-timedOutCtx.Done():
			// Timeout means the distro is not reachable.
			return nil, newUnreachableDistroErr(errors.New("timed out waiting for client"))
		case <-time.After(tickRate):
			conn := w.Connection()

			if conn == nil {
				tickRate = time.Second
				continue
			}

			return conn, nil
		}
	}
}
