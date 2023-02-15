package distro

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/ubuntu/decorate"
)

// Task represents a given task that could be retried to dispatch to GRPC.
type Task interface {
	Execute(context.Context, wslserviceapi.WSLClient) error
	fmt.Stringer
	ShouldRetry() bool
}

// startProcessingTasks starts the main task processing goroutine.
func (d *Distro) startProcessingTasks(ctx context.Context) {
	log.Debugf(ctx, "Distro %q: starting task processing", d.Name)

	d.canProcessTasks = make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	go func() { d.processTasks(ctx) }()
	d.cancel = cancel
}

// stopProcessingTasks stops the main task processing goroutine and wait for it to be done.
func (d *Distro) stopProcessingTasks(ctx context.Context) error {
	log.Debugf(ctx, "Distro %q: stopping task processing", d.Name)
	if d.canProcessTasks == nil {
		return errors.New("could not stop tasks: task processing is not running.")
	}
	d.cancel()
	<-d.canProcessTasks
	d.canProcessTasks = nil
	log.Debugf(ctx, "Distro %q: stopped task processing", d.Name)
	return nil
}

// SubmitTask enqueue a new task on our current worker list.
// It will return an error in these cases:
// - The queue is full
// - The distro has been cleaned up.
func (d *Distro) SubmitTask(t Task) (err error) {
	defer decorate.OnError(&err, "distro %q: task %q: could not submit", d.Name, t)

	if d.canProcessTasks == nil {
		return errors.New("task processing is not running.")
	}

	log.Infof(context.TODO(), "Distro %q: Submitting task %q to queue", d.Name, t)
	select {
	case d.tasks <- t:
	default:
		return errors.New("queue is full")
	}
	return nil
}

// processTasks is the main loop for the distro, processing any existing tasks while starting and releasing
// locks to distro,.
func (d *Distro) processTasks(ctx context.Context) {
	defer close(d.canProcessTasks)

	for d.UnreachableErr == nil {
		select {
		case <-ctx.Done():
			return
		case t := <-d.tasks:
			if err := d.processSingleTask(ctx, t); err != nil {
				log.Debugf(context.TODO(), "Distro %q: task %q: %v", d, t, err)
			}
		}
	}
}

func (d *Distro) processSingleTask(ctx context.Context, t Task) error {
	log.Debugf(context.TODO(), "Distro %q: task %q: dequeued", d.Name, t)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := d.keepAwake(ctx)
	if err != nil {
		_ = d.SubmitTask(t) // requeue the task for good measure, we will purge it anyway.
		d.UnreachableErr = err
		return errors.New("task could not start task: could not wake distro up")
	}
	log.Debugf(context.TODO(), "Distro %q: task %q: distro is active.", d.Name, t)

	// ensure/wait distro is active + timeout when
	// TODO: test stopping a service, which is marked Restart=Always, to ensure that only disabling the service really mark it as invalid.
	// TODO: test wsl --shutdown with a sleep here.
	// FIXME TODO FIXME
	client, err := d.waitForClient(ctx)
	if err != nil {
		_ = d.SubmitTask(t) // requeue the task for good measure, we will purge it anyway.
		d.UnreachableErr = err
		return errors.New("task could not start task: could not contact distro")
	}
	log.Debugf(context.TODO(), "Distro %q: task %q: connection to distro established, running task.", d.Name, t)

	for {
		// Avoid retrying if the task failed due to a cancelled or timed out context
		// It also avoids executing in the much rarer case that we cancel or time out right after getting the client
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err = t.Execute(ctx, client)
		if err != nil && t.ShouldRetry() {
			log.Debugf(ctx, "Distro %q: task %q: retrying after obtaining error: %v", d.Name, t, err)
			continue
		}

		// No retry: abandon task potentially in error.
		if err != nil {
			return fmt.Errorf("task errored out: %v", err)
		}

		log.Debugf(context.TODO(), "Distro %q: task %q: task completed successfully", d.Name, t)
		break
	}

	return nil
}

// waitForClient waits for a valid GRPC client to connect to. It will retry for a while before
// erroring out.
func (d *Distro) waitForClient(ctx context.Context) (wslserviceapi.WSLClient, error) {
	timedOutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	for {
		select {
		case <-timedOutCtx.Done():
			return nil, timedOutCtx.Err()
		case <-time.After(1 * time.Second):
			client := d.Client()

			if client == nil {
				log.Debugf(ctx, "Distro %q: client not available yet\n", d.Name)
				continue
			}
			return client, nil
		}
	}
}
