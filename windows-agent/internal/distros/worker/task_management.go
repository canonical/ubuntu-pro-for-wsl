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
	ID   uint64
	Skip bool
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

		for _, queued := range tm.tasks {
			if !task.Is(t.Task, queued.Task) {
				continue
			}
			queued.Skip = true
		}

		tm.tasks = append(tm.tasks, t)

		select {
		case tm.queue <- t:
		default:
			return errors.New("queue is full")
		}
	}

	return tm.save()
}

func (tm *taskManager) nextTask(ctx context.Context) (t *managedTask, err error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case t = <-tm.queue:
		}

		if !t.Skip {
			return t, nil
		}
	}
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

	// Task failed during execution, nothing else to be done.
	log.Errorf(context.TODO(), "Task %s failed: %v", *t, errResult)

	return nil
}

func (tm *taskManager) save() (err error) {
	defer decorate.OnError(&err, "could not save current work in progress")

	var tasks []task.Task
	for i := range tm.tasks {
		if tm.tasks[i].Skip {
			continue
		}
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

	if err := tm.submit(tasks...); err != nil {
		return err
	}

	return nil
}
