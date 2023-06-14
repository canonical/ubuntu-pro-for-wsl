// Package task exposes the Task interface and some utils related to it.
package task

import (
	"context"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

// Task represents a given task that is ging to be executed by a distro.
type Task interface {
	Execute(context.Context, wslserviceapi.WSLClient) error
}

// taskWithIs are tasks that implement the Is method as a custom comparator.
type taskWithIs interface {
	Task
	Is(t Task) bool
}

// Is compares to tasks to determine if they match.
//
// A task is considered to match a target if it is equal to that target or if
// it implements a method Is(Task) bool such that Is(target) returns true.
func Is(t, target Task) bool {
	if T, ok := t.(taskWithIs); ok {
		return T.Is(target)
	}
	return t == target
}

// NeedsRetryError is an error that should be emitted by tasks that, in case of failure,
// should be retried at the next startup sequence.
type NeedsRetryError struct {
	err      error
	taskName string
}

// NewNeedsRetryError constructs a NeedsRetryError. This error can be used to signal the
// task manager that the task needs be retried upon restarting the agent.
func NewNeedsRetryError(t Task, err error) error {
	return NeedsRetryError{
		err:      err,
		taskName: fmt.Sprintf("%s", t),
	}
}

func (e NeedsRetryError) Error() string {
	return fmt.Sprintf("task %q needs to be retried: %v", e.taskName, e.err)
}
