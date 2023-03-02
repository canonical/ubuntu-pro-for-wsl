package distro

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/ubuntu/decorate"
)

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
		return errors.New("could not stop tasks: task processing is not running")
	}
	d.cancel()
	<-d.canProcessTasks
	d.canProcessTasks = nil
	log.Debugf(ctx, "Distro %q: stopped task processing", d.Name)
	return nil
}

// SubmitTasks enqueue a new task on our current worker list.
// It will return an error in these cases:
// - The queue is full
// - The distro has been cleaned up.
func (d *Distro) SubmitTasks(tasks ...task.Task) (err error) {
	defer decorate.OnError(&err, "distro %q: tasks %q: could not submit", d.Name, tasks)

	if d.UnreachableErr != nil {
		return d.UnreachableErr
	}

	if d.canProcessTasks == nil {
		return errors.New("task processing is not running")
	}

	if len(tasks) == 0 {
		return nil
	}

	log.Infof(context.TODO(), "Distro %q: Submitting tasks %q to queue", d.Name, tasks)
	return d.taskManager.submit(tasks...)
}

// processTasks is the main loop for the distro, processing any existing tasks while starting and releasing
// locks to distro,.
func (d *Distro) processTasks(ctx context.Context) {
	defer close(d.canProcessTasks)

	for d.UnreachableErr == nil {
		select {
		case <-ctx.Done():
			return
		case t := <-d.taskManager.queue:
			err := d.processSingleTask(ctx, *t)
			err = d.taskManager.done(t, err)
			if err != nil {
				log.Errorf(ctx, "distro %q: %v", d.Name, err)
			}
		}
	}
}

func (d *Distro) processSingleTask(ctx context.Context, t managedTask) error {
	log.Debugf(context.TODO(), "Distro %q: task %q: dequeued", d.Name, t)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := d.keepAwakeAndWaitForClient(ctx)
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
			log.Debugf(context.TODO(), "Distro %q: task %q: task completed successfully", d.Name, t)
			break
		}

		// No retry: abandon task regardless of error result.
		if !t.ShouldRetry() {
			return err
		}

		log.Warningf(ctx, "Distro %q: task %q: retrying after obtaining error: %v", d.Name, t, err)
	}

	return nil
}

func (d *Distro) keepAwakeAndWaitForClient(ctx context.Context) (client wslserviceapi.WSLClient, err error) {
	log.Debugf(context.TODO(), "Distro %q: ensuring active connection.", d.Name)

	defer func() {
		if err != nil {
			d.UnreachableErr = err
		}
	}()

	for i := 0; i < 5; i++ {
		client, err = func() (wslserviceapi.WSLClient, error) {
			ctx, cancel := context.WithCancel(ctx)
			defer func() {
				if err != nil {
					// On success, the caller will cancel the parent context
					return
				}
				// On error, we avoid stacking keepAwake calls with each retry.
				cancel()
			}()

			err := d.keepAwake(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not wake distro up: %v", err)
			}

			log.Debugf(context.TODO(), "Distro %q: distro is running.", d.Name)

			client, err := d.waitForClient(ctx)
			if err != nil {
				return nil, errors.New("could not contact distro")
			}

			log.Debugf(context.TODO(), "Distro %q: connection is active.", d.Name)
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
func (d *Distro) waitForClient(ctx context.Context) (wslserviceapi.WSLClient, error) {
	timedOutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tickRate := 0 * time.Second
	for {
		select {
		case <-timedOutCtx.Done():
			log.Warningf(ctx, "Distro %q: timed out waiting for client\n", d.Name)
			return nil, timedOutCtx.Err()
		case <-time.After(tickRate):
			client := d.Client()

			if client == nil {
				tickRate = time.Second
				continue
			}

			return client, nil
		}
	}
}
