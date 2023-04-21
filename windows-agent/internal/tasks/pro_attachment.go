// Package tasks implements tasks to be submitted to distros.
package tasks

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

func init() {
	task.Register[ProAttachment]()
}

// ProAttachment is a task that attaches/dettaches Ubuntu Pro to a distro:
// - to attach: send the token to attach with.
// - to detach: send an empty token.
type ProAttachment struct {
	Token string
}

// Execute is needed to fulfil Task.
func (t ProAttachment) Execute(ctx context.Context, client wslserviceapi.WSLClient) error {
	_, err := client.ApplyProToken(context.TODO(), &wslserviceapi.ProAttachInfo{Token: t.Token})
	return err
}

// String is needed to fulfil Task.
func (t ProAttachment) String() string {
	return "ProAttachment"
}

// ShouldRetry is needed to fulfil Task.
func (t ProAttachment) ShouldRetry() bool {
	return false
}

// Is is a custom comparator. All AttachPro tasks are considered equivalent.
func (t ProAttachment) Is(other task.Task) bool {
	_, ok := other.(ProAttachment)
	return ok
}
