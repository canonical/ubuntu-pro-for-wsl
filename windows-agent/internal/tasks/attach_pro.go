// Package tasks implements tasks to be submitted to distros.
package tasks

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

// AttachPro is a task that sends a token to a distro so it can
// attach itself to Ubuntu Pro.
type AttachPro struct {
	Token string
}

// Execute is needed to fulfil Task.
func (t AttachPro) Execute(ctx context.Context, client wslserviceapi.WSLClient) error {
	_, err := client.ProAttach(context.TODO(), &wslserviceapi.AttachInfo{Token: t.Token})
	return err
}

// String is needed to fulfil Task.
func (t AttachPro) String() string {
	return "AttachPro"
}

// ShouldRetry is needed to fulfil Task.
func (t AttachPro) ShouldRetry() bool {
	return false
}

// Is is a custom comparator. All AttachPro tasks are considered equivalent.
func (t AttachPro) Is(other task.Task) bool {
	_, ok := other.(AttachPro)
	return ok
}
