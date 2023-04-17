// Package initialtasks keeps track of tasks that must be performed on all
// new distros.
package initialtasks

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

// All returns a copy of all the tasks in the list of initial tasks.
func (i *InitialTasks) All() (tasks []task.Task) {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	tasks = make([]task.Task, len(i.tasks))
	copy(tasks, i.tasks)
	log.Debugf(context.TODO(), "Returning all initial tasks: %q", tasks)

	return tasks
}

// Add appends a task to the list of initial tasks.
func (i *InitialTasks) Add(ctx context.Context, t task.Task) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	log.Infof(ctx, "Adding %q to list of initial tasks", t)
	i.tasks = removeDuplicates(i.tasks, t)
	i.tasks = append(i.tasks, t)

	if err := i.save(); err != nil {
		return err
	}

	return nil
}

// Remove drops a task from the list of initial tasks. task.Is(t, target) is used to
// identify the task.
func (i *InitialTasks) Remove(ctx context.Context, t task.Task) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	idx := slices.IndexFunc(i.tasks, func(target task.Task) bool { return task.Is(t, target) })
	if idx == -1 {
		log.Infof(ctx, "task %q is not in the init task list. Ignoring removal.", t)
		return nil
	}

	log.Infof(ctx, "Removing %q to list of initial tasks", t)
	i.tasks = slices.Delete(i.tasks, idx, idx+1)

	if err := i.save(); err != nil {
		return fmt.Errorf("removal of task %q from the init list: %w", t, err)
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

// removeDuplicates removes all tasks that are the same as the target.
//
// In order to determine equality, tasks.Is(target, task[i]) is used.
func removeDuplicates(tasks []task.Task, target task.Task) []task.Task {
	if len(tasks) == 0 {
		return tasks
	}

	// Partition algorithm
	//
	// Split the array into two intervals [0, p) and [p, end) such that tasks.Is(target, tasks[i])
	// is false for all entries in the first interval (i<p), and true for all entries in the second one (i>=p).
	var p int
	for i := range tasks {
		if task.Is(target, tasks[i]) {
			continue
		}
		if i == p {
			p++
			continue
		}
		tasks[i], tasks[p] = tasks[p], tasks[i]
		p++
	}

	// End of partition, remove task duplicates
	return tasks[:p]
}
