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

type taskManager struct {
	storagePath string

	tasks []*managedTask
	queue chan *managedTask

	largestID uint64

	mu sync.Mutex
}

func newTaskManager(ctx context.Context, storagePath string) (*taskManager, error) {
	tm := taskManager{
		storagePath: storagePath,
		queue:       make(chan *managedTask, taskQueueSize),
	}

	if err := tm.load(ctx); err != nil {
		return &tm, err
	}
	return &tm, nil
}

func (tm *taskManager) submit(deferred bool, tasks ...task.Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.submitUnsafe(deferred, tasks...)
}

// must be used under a mutex
func (tm *taskManager) submitUnsafe(deferred bool, tasks ...task.Task) error {
	for i := range tasks {
		tm.largestID++
		t := &managedTask{
			ID:   tm.largestID,
			Task: tasks[i],
		}

		tm.tasks = append(tm.tasks, t)
		if deferred {
			// deferred tasks will be queued next time load() is called
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

func (tm *taskManager) nextTask(ctx context.Context) (*managedTask, bool) {
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

	select {
	case <-ctx.Done():
		return nil, false
	case t := <-tm.queue:
		// Remove task from list
		tm.mu.Lock()
		defer tm.mu.Unlock()

		idx := slices.Index(tm.tasks, t)
		if idx != -1 {
			tm.tasks = slices.Delete(tm.tasks, idx, idx+1)
		}

		if err := tm.save(); err != nil {
			log.Errorf(ctx, "could not write task list to disk: %v", err)
			return t, false
		}

		return t, true
	}
}

func (tm *taskManager) taskDone(ctx context.Context, t *managedTask, taskResult error) (err error) {
	decorate.OnError(&err, "task %s", t)

	if taskResult == nil {
		// Succesful task: nothing to do
		return nil
	}

	log.Errorf(ctx, "%v", taskResult)

	if !errors.As(taskResult, &task.NeedsRetryError{}) {
		// Task failed but does not need re-submission
		return nil
	}

	// Task is resubmited as deferred
	if err := tm.submit(true, t.Task); err != nil {
		return err
	}

	return nil
}

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

func (tm *taskManager) load(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not load previous work in progress")

	tm.mu.Lock()
	defer tm.mu.Unlock()

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
