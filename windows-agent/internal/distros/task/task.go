// Package task exposes the Task interface and some utils related to it.
package task

import (
	"context"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/wslserviceapi"
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
	SourceErr error
}

func (e NeedsRetryError) Error() string {
	return fmt.Sprintf("task marked for retrial: %v", e.SourceErr)
}
