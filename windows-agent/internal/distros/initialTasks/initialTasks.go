// Package initialTasks keeps track of tasks that must be performed on all
// new distros.
package initialTasks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"golang.org/x/exp/slices"
)

const (
	initialTasksFileName = "Initial tasks.tasks"
)

// InitialTasks contains the tasks that are to be done by a distro when it
// first contacts the agent.
type InitialTasks struct {
	tasks []task.Task

	storagePath string
	mu          sync.RWMutex
}

// New constructs an InitialTasks and loads its task from disk.
func New(storageDir string) (*InitialTasks, error) {
	init := InitialTasks{
		storagePath: filepath.Join(storageDir, initialTasksFileName),
	}

	out, err := os.ReadFile(init.storagePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &init, nil
		}
		return nil, err
	}

	if init.tasks, err = task.UnmarshalYAML(out); err != nil {
		return nil, err
	}

	return &init, nil
}

// GetAll returns a copy of all the tasks in the list of initial tasks.
func (i *InitialTasks) GetAll() (tasks []task.Task) {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	tasks = make([]task.Task, len(i.tasks))
	copy(tasks, i.tasks)
	log.Debugf(context.TODO(), "Requested all initial tasks: %q", tasks)

	return tasks
}

// Add appends a task to the list of initial tasks.
func (i *InitialTasks) Add(t task.Task) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.tasks = append(i.tasks, t)

	if err := i.save(); err != nil {
		return err
	}

	return nil
}

// Remove drops a task from the list of initial tasks. task.Is(t, target) is used to
// identify the task.
func (i *InitialTasks) Remove(ctx context.Context, target task.Task) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	idx := slices.IndexFunc(i.tasks, func(t task.Task) bool { return task.Is(target, t) })
	if idx != -1 {
		log.Infof(ctx, "task %q is not in the init task list. Ignoring removal.", target)
		return nil
	}
	i.tasks = slices.Delete(i.tasks, idx, idx+1)

	if err := i.save(); err != nil {
		return fmt.Errorf("removal of task %q from the init list: %w", target, err)
	}
	return nil
}

// save stores the contents of the task list to disk.
func (i *InitialTasks) save() (err error) {
	defer decorate.OnError(&err, "could not save new init list")

	out, err := task.MarshalYAML(i.tasks)
	if err != nil {
		return err
	}

	if err = os.WriteFile(i.storagePath+".new", out, 0600); err != nil {
		return err
	}

	if err = os.Rename(i.storagePath+".new", i.storagePath); err != nil {
		return err
	}

	return nil
}
