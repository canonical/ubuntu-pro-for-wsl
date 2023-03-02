package distro

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

func (tm *taskManager) submit(tasks ...task.Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for i := range tasks {
		tm.largestID++
		t := &managedTask{
			ID:   tm.largestID,
			Task: tasks[i],
		}

		select {
		case tm.queue <- t:
		default:
			return errors.New("queue is full")
		}

		tm.tasks = append(tm.tasks, t)
	}
	return tm.save()
}

func (tm *taskManager) done(t *managedTask, errResult error) (err error) {
	decorate.OnError(&err, "task %s", t)

	tm.mu.Lock()

	idx := slices.Index(tm.tasks, t)
	tm.tasks = slices.Delete(tm.tasks, idx, idx+1)

	if err = tm.save(); err != nil {
		tm.mu.Unlock()
		return err
	}

	tm.mu.Unlock()

	// Task succeeded.
	if errResult == nil {
		return
	}

	// Task failed during attempt to connect to distro, resubmit.
	var target taskExecutionError
	if !errors.As(errResult, &target) {
		return tm.submit(t)
	}

	// Task failed during execution, nothing else to be done.
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

	tm.queue = make(chan *managedTask)
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

	if err := tm.submit(tasks...); err != nil {
		return err
	}

	return nil
}
