// Package tasks implements tasks to be submitted to distros.
package tasks

import (
	"context"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
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
func (t ProAttachment) Execute(ctx context.Context, conn task.Connection) error {
	err := conn.SendProAttachment(t.Token)
	if err != nil {
		return task.NeedsRetryError{SourceErr: err}
	}
	return nil
}

// String is needed to fulfil Task.
func (t ProAttachment) String() string {
	return fmt.Sprintf("%T task with token: %s", t, common.Obfuscate(t.Token))
}

// Is is a custom comparator. All ProAttachment tasks are considered equivalent.
func (t ProAttachment) Is(other task.Task) bool {
	_, ok := other.(ProAttachment)
	return ok
}
