// Package task implements tasks to be submitted to distros.
package task

import (
	"context"

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
