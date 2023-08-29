package worker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"golang.org/x/exp/slices"
)

// taskQueueSize is the maximum amount of tasks a queue is allowed to hold.
const taskQueueSize = 100

// managedTask is a type that carries a task with it, with added metadata and functionality to
// serialize and deserialize.
type managedTask struct {
	ID uint64
	task.Task
}

func (m managedTask) String() string {
	return fmt.Sprintf("Task #%d (%T)", m.ID, m.Task)
}

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

	tasks []*managedTask
	queue chan *managedTask

	largestID uint64

	mu sync.RWMutex
}

// newTaskManager constructs and initializes a TaskManager.
func newTaskManager(ctx context.Context, storagePath string) (*taskManager, error) {
	tm := taskManager{
		storagePath: storagePath,
		queue:       make(chan *managedTask, taskQueueSize),
	}

	if err := tm.Load(ctx); err != nil {
		return &tm, err
	}
	return &tm, nil
}

// QueueLen returns the length of the task queue containing non-deferred tasks.
func (tm *taskManager) QueueLen() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.queue)
}

// TaskLen returns the length of the task queue plus the deferred task queue.
func (tm *taskManager) TaskLen() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.tasks)
}

// Submit adds a task
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

	for i := range tasks {
		tm.largestID++
		t := &managedTask{
			ID:   tm.largestID,
			Task: tasks[i],
		}

		tm.tasks = append(tm.tasks, t)
		if deferred {
			// deferred tasks will be queued next time load() is called.
			continue
		}

		select {
		case tm.queue <- t:
		default:
			return errors.New("queue is full")
		}
	}

	return tm.save()
}

// NextTask pulls the next task from the queue. If no task is queued, this function blocks until either a task is
// submitted or the context is cancelled, whichever happens first.
// The second argument indicates whether a task was pulled or not.
func (tm *taskManager) NextTask(ctx context.Context) (*managedTask, bool) {
	// This double-select gives priority to the context over the manager queue. Not very
	// important in production code but it makes the code more predictable for testing.
	//
	// Without this, there is always a chance that the worker will select the task
	// channel rather than the context.Done.
	select {
	case <-ctx.Done():
		return nil, false
	default:
	}

	// Avoid races with Load()
	tm.mu.RLock()
	queue := tm.queue
	tm.mu.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return nil, false
		case t, open := <-queue:

			if !open {
				// There was a reload: we need to refresh the queue after the Load is completed
				// This happens because Load() creates a new queue (closing the old one), so
				// we have to start reading from the new one.
				tm.mu.RLock()
				queue = tm.queue
				tm.mu.RUnlock()

				continue
			}

			tm.mu.Lock()
			defer tm.mu.Unlock()

			// Remove task from list
			idx := slices.Index(tm.tasks, t)
			if idx != -1 {
				tm.tasks = slices.Delete(tm.tasks, idx, idx+1)
			}

			if err := tm.save(); err != nil {
				log.Errorf(ctx, "NextTask: could not write task list to disk: %v", err)
				return t, false
			}

			return t, true
		}
	}
}

// TaskDone cleans up after a task is completed, and conditionally re-submits failed ones.
func (tm *taskManager) TaskDone(ctx context.Context, t *managedTask, taskResult error) (err error) {
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

	// Task is resubmited as deferred
	if err := tm.Submit(true, t.Task); err != nil {
		return err
	}

	return nil
}

// save writes the current task queue (plus deferred tasks) to file.
func (tm *taskManager) save() (err error) {
	defer decorate.OnError(&err, "could not save current work in progress")

	var tasks []task.Task
	for i := range tm.tasks {
		tasks = append(tasks, tm.tasks[i].Task)
	}

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
func (tm *taskManager) Load(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not load previous work in progress")

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Dump old queue and reload from file
	close(tm.queue)

	tm.tasks = make([]*managedTask, 0)
	tm.queue = make(chan *managedTask, taskQueueSize)
	tm.largestID = 0

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

	if len(tasks) >= taskQueueSize {
		excess := taskQueueSize - len(tasks)
		log.Warningf(ctx, "dropped %d tasks because at most %d can be queued up", excess, taskQueueSize)
		tasks = tasks[:taskQueueSize]
	}

	if err := tm.submitUnsafe(false, tasks...); err != nil {
		return err
	}

	return nil
}
