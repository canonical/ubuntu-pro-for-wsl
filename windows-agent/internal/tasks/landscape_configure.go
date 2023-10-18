package tasks

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

func init() {
	task.Register[LandscapeConfigure]()
}

// LandscapeConfigure is a task that registers/disables Landscape in a distro:
// - to register: send the config to register with.
// - to disable: send an empty config.
type LandscapeConfigure struct {
	Config string
}

// Execute sends the config to the target WSL-Pro-Service so that the distro can be
// registered in Landscape.
func (t LandscapeConfigure) Execute(ctx context.Context, client wslserviceapi.WSLClient) error {
	// First value is a dummy message, we ignore it. We only care about success/failure.
	_, err := client.ApplyLandscapeConfig(ctx, &wslserviceapi.LandscapeConfig{Configuration: t.Config})
	if err != nil {
		return task.NeedsRetryError{SourceErr: err}
	}
	return nil
}

// String returns the name of the task.
func (t LandscapeConfigure) String() string {
	return "LandscapeConfigure"
}

// Is is a custom comparator. All LandscapeConfigure tasks are considered equivalent. In other words: newer
// instructions to configure will override old ones.
func (t LandscapeConfigure) Is(other task.Task) bool {
	_, ok := other.(LandscapeConfigure)
	return ok
}
