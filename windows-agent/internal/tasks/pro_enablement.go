// Package tasks implements tasks to be submitted to distros.
package tasks

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

func init() {
	task.Register[ProEnablement]()
}

// ProEnablement is a task that enables Ubuntu Pro to a distro:
type ProEnablement struct {
	// Service is any of the pro services such as esm-apps, esm-infra, livepatch, etc.
	Service string

	// Enable must be set to true to enable, and false to disable.
	Enable bool
}

// Execute is needed to fulfil Task.
func (t ProEnablement) Execute(ctx context.Context, client wslserviceapi.WSLClient) error {
	_, err := client.ProServiceEnablement(context.TODO(), &wslserviceapi.ProService{Service: t.Service, Enable: t.Enable})
	return err
}

// String is needed to fulfil Task.
func (t ProEnablement) String() string {
	return "ProEnable"
}

// ShouldRetry is needed to fulfil Task.
func (t ProEnablement) ShouldRetry() bool {
	return false
}

// Is is a custom comparator. ProEnablement tasks are considered equivalent if they target the same service.
func (t ProEnablement) Is(other task.Task) bool {
	o, ok := other.(ProEnablement)
	if !ok {
		return false
	}

	return o.Service == t.Service
}
