package worker

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"sync"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
)

// taskmanager is a helper struct for the worker that manages task submission
// and completion management, as well as its disk storage.
//
// The worker should only ever call public methods of this struct, and should
// not read or write into any of its private fields.
//
// The only private method that should be used by the worker is newTaskManager,
// which is set to private because it is a freestanding function and we don't
// want outside packages to be able to use it.
type taskManager struct {
	storagePath string

	tasks         *taskQueue
	deferredTasks *taskQueue

	mu sync.RWMutex
}

// newTaskManager constructs and initializes a TaskManager.
func newTaskManager(storagePath string) (*taskManager, error) {
	tm := taskManager{
		storagePath:   storagePath,
		tasks:         newTaskQueue(),
		deferredTasks: newTaskQueue(),
	}

	if err := tm.load(); err != nil {
		return &tm, err
	}
	return &tm, nil
}

// QueueLen returns the length of the task queue containing non-deferred tasks.
func (tm *taskManager) QueueLen() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.tasks.Len()
}

// TaskLen returns the length of the task queue plus the deferred task queue.
func (tm *taskManager) TaskLen() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.tasks.Len() + tm.deferredTasks.Len()
}

// Submit adds a task with high priority, meaning that any equivalent task will
// be removed from the queue.
//
// If deferred is set to true, task execution is deferred until the next load()
// Otherwise, it is added to the queue immediately.
func (tm *taskManager) Submit(deferred bool, tasks ...task.Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.submitUnsafe(deferred, tasks...)
}

// submitUnsafe is the thread-unsafe version of Submit.
func (tm *taskManager) submitUnsafe(deferred bool, tasks ...task.Task) (err error) {
	defer decorate.OnError(&err, "could not submit task")

	thisQueue := &tm.tasks
	otherQueue := &tm.deferredTasks
	if deferred {
		thisQueue, otherQueue = otherQueue, thisQueue
	}

	for i := range tasks {
		(*otherQueue).Remove(tasks[i])
		(*thisQueue).Push(tasks[i])
	}

	return tm.save()
}

// resubmit submits a task with lowest priority, meaning that it will be overridden
// by any equivalent already in the queue.
func (tm *taskManager) resubmit(t task.Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.tasks.Contains(t) {
		// No need to resubmit
		return nil
	}
	tm.deferredTasks.PushIfNew(t)

	return tm.save()
}

// NextTask pulls the next task from the queue. If no task is queued, this function blocks until either a task is
// submitted or the context is cancelled, whichever happens first.
// The second argument indicates whether a task was pulled or not.
func (tm *taskManager) NextTask(ctx context.Context) (task.Task, bool) {
	t := tm.tasks.Pull(ctx)
	return t, t != nil
}

// TaskDone cleans up after a task is completed, and conditionally re-submits failed ones.
func (tm *taskManager) TaskDone(ctx context.Context, t task.Task, taskResult error) (err error) {
	decorate.OnError(&err, "task %s", t)

	if taskResult == nil {
		// Successful task: nothing to do
		return nil
	}

	log.Errorf(ctx, "%v", taskResult)

	if !errors.As(taskResult, &task.NeedsRetryError{}) {
		// Task failed but does not need re-submission
		return nil
	}

	return tm.resubmit(t)
}

// EnqueueDeferredTasks takes all deferred tasks and promotes them
// to regular tasks.
func (tm *taskManager) EnqueueDeferredTasks() {
	tm.tasks.Absorb(tm.deferredTasks)
}

// save writes the current task queue (plus deferred tasks) to file.
func (tm *taskManager) save() (err error) {
	defer decorate.OnError(&err, "could not save current work in progress")

	tasks := append(tm.tasks.Data(), tm.deferredTasks.Data()...)

	out, err := task.MarshalYAML(tasks)
	if err != nil {
		return err
	}

	if err = os.WriteFile(tm.storagePath+".new", out, 0600); err != nil {
		return err
	}

	if err = os.Rename(tm.storagePath+".new", tm.storagePath); err != nil {
		return err
	}

	return nil
}

// Load loads tasks from file.
func (tm *taskManager) load() (err error) {
	defer decorate.OnError(&err, "could not load previous work in progress")

	tm.mu.Lock()
	defer tm.mu.Unlock()

	out, err := os.ReadFile(tm.storagePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var tasks []task.Task
	if tasks, err = task.UnmarshalYAML(out); err != nil {
		return err
	}

	tm.tasks.Load(tasks)

	return nil
}
