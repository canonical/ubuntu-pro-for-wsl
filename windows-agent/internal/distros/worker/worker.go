// Package worker manages the execution and queue of tasks.
package worker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

type distro interface {
	Name() string
	KeepAwake(context.Context) error

	IsValid() bool
	Invalidate(error)
}

// Worker contains all the logic around task queueing and execution for one particular distro.
type Worker struct {
	distro  distro
	manager *taskManager

	cancel     context.CancelFunc
	processing chan struct{}

	conn   *grpc.ClientConn
	connMu sync.RWMutex
}

type options struct {
	conf *config.Config
}

// Option is an optional argument for worker.New.
type Option func(*options)

// WithConfig is an optional parameter for worker.New that allows for
// conditionally importing the provisioning tasks.
func WithConfig(conf *config.Config) Option {
	return func(o *options) {
		o.conf = conf
	}
}

// New creates a new worker and starts it. Call Stop when you're done to avoid leaking the task execution goroutine.
func New(ctx context.Context, d distro, storageDir string, args ...Option) (*Worker, error) {
	storagePath := filepath.Join(storageDir, d.Name()+".tasks")

	var opts options
	for _, f := range args {
		f(&opts)
	}

	tm, err := newTaskManager(ctx, storagePath)
	if err != nil {
		return nil, err
	}

	w := &Worker{
		distro:  d,
		manager: tm,
	}

	w.start(ctx)

	// load and submit provisioning tasks. (case of first contact with distro)
	if opts.conf == nil {
		return w, nil
	}

	provisioning, err := opts.conf.ProvisioningTasks(ctx)
	if err != nil {
		return w, err
	}

	if err := w.SubmitTasks(provisioning...); err != nil {
		w.Stop(ctx)
		return nil, err
	}

	return w, nil
}

// IsActive returns true when the worker is running, and there exists an active
// connection to its GRPC service.
func (w *Worker) IsActive() bool {
	return w.Client() != nil
}

// Client returns the client to the WSL task service.
// Client returns nil when no connection is set up.
func (w *Worker) Client() wslserviceapi.WSLClient {
	w.connMu.RLock()
	defer w.connMu.RUnlock()

	if w.conn == nil {
		return nil
	}

	return wslserviceapi.NewWSLClient(w.conn)
}

// SetConnection removes the connection associated with the distro.
func (w *Worker) SetConnection(conn *grpc.ClientConn) {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	if w.conn != nil {
		if err := w.conn.Close(); err != nil {
			log.Warningf(context.TODO(), "distro %q: could not close previous grpc connection: %v", w.distro.Name(), err)
		}
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

// SubmitTasks enqueues one or more task on our current worker list.
// It will return an error in these cases:
// - The queue is full
// - The distro has been cleaned up.
func (w *Worker) SubmitTasks(tasks ...task.Task) (err error) {
	defer decorate.OnError(&err, "distro %q: tasks %q: could not submit", w.distro.Name(), tasks)

	if len(tasks) == 0 {
		return nil
	}

	log.Infof(context.TODO(), "Distro %q: Submitting tasks %q to queue", w.distro.Name(), tasks)
	return w.manager.submit(tasks...)
}

// processTasks is the main loop for the distro, processing any existing tasks while starting and releasing
// locks to distro,.
func (w *Worker) processTasks(ctx context.Context) {
	defer close(w.processing)

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-w.manager.queue:
			resultErr := w.processSingleTask(ctx, *t)

			var target unreachableDistroError
			if errors.Is(resultErr, &target) {
				w.distro.Invalidate(target)
				continue
			}

			err := w.manager.done(t, resultErr)
			if err != nil {
				log.Errorf(ctx, "distro %q: %v", w.distro.Name(), err)
			}
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

func (w *Worker) processSingleTask(ctx context.Context, t managedTask) error {
	log.Debugf(context.TODO(), "Distro %q: task %q: dequeued", w.distro.Name(), t)

	if !w.distro.IsValid() {
		return newUnreachableDistroErr(errors.New("distro marked as invalid"))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := w.keepAwakeAndWaitForClient(ctx)
	if err != nil {
		return fmt.Errorf("task %v: could not start task: %v", t, err)
	}

	for {
		// Avoid retrying if the task failed due to a cancelled or timed out context
		// It also avoids executing in the much rarer case that we cancel or time out right after getting the client
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err = t.Execute(ctx, client)
		if err == nil {
			log.Debugf(context.TODO(), "Distro %q: task %q: task completed successfully", w.distro.Name(), t)
			break
		}

		// No retry: abandon task regardless of error result.
		if !t.ShouldRetry() {
			return err
		}

		log.Warningf(ctx, "Distro %q: task %q: retrying after obtaining error: %v", w.distro.Name(), t, err)
	}

	return nil
}

func (w *Worker) keepAwakeAndWaitForClient(ctx context.Context) (client wslserviceapi.WSLClient, err error) {
	log.Debugf(context.TODO(), "Distro %q: ensuring active connection.", w.distro.Name())

	for i := 0; i < 5; i++ {
		client, err = func() (wslserviceapi.WSLClient, error) {
			ctx, cancel := context.WithCancel(ctx)
			defer func() {
				if err == nil {
					// On success, the caller will cancel the parent context
					return
				}
				// On error, we avoid stacking keepAwake calls with each retry.
				cancel()
			}()

			err := w.distro.KeepAwake(ctx)
			if err != nil {
				return nil, newUnreachableDistroErr(err)
			}

			log.Debugf(context.TODO(), "Distro %q: distro is running.", w.distro.Name())

			client, err := w.waitForClient(ctx)
			if err != nil {
				return nil, newUnreachableDistroErr(err)
			}

			log.Debugf(context.TODO(), "Distro %q: connection is active.", w.distro.Name())
			return client, nil
		}()

		if err == nil {
			break
		}
	}

	return client, err
}

// waitForClient waits for a valid GRPC client to connect to. It will retry for a while before
// erroring out.
func (w *Worker) waitForClient(ctx context.Context) (wslserviceapi.WSLClient, error) {
	timedOutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tickRate := 0 * time.Second
	for {
		select {
		case <-timedOutCtx.Done():
			log.Warningf(ctx, "Distro %q: timed out waiting for client\n", w.distro.Name())
			return nil, fmt.Errorf("when waiting for client: %v", timedOutCtx.Err())
		case <-time.After(tickRate):
			client := w.Client()

			if client == nil {
				tickRate = time.Second
				continue
			}

			return client, nil
		}
	}
}
