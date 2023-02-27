// Package task exposes the Task interface and some utils related to it.
package task

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

// Task represents a given task that is ging to be executed by a distro.
// Execute is the job to be done, and ShouldRetry should not return true forever,
// and rather contain some logic to stop retrying at some point.
type Task interface {
	Execute(context.Context, wslserviceapi.WSLClient) error
	ShouldRetry() bool
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
