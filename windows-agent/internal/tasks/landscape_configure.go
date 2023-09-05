package tasks

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
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

// NewLandscapeConfigure creates a LandscapeConfigure. It overrides the name of the Landscape
// client so it matches the name of the distro.
func NewLandscapeConfigure(ctx context.Context, Config string, distroName string) (conf LandscapeConfigure, err error) {
	defer decorate.OnError(&err, "NewLandscapeConfigure error")

	if Config == "" {
		// Landscape disablement
		return LandscapeConfigure{}, nil
	}

	r := strings.NewReader(Config)
	data, err := ini.Load(r)
	if err != nil {
		return LandscapeConfigure{}, fmt.Errorf("could not parse config: %v", err)
	}

	const section = "client"
	s, err := data.GetSection(section)
	if err != nil {
		return LandscapeConfigure{}, fmt.Errorf("could not find [%s] section: %v", section, err)
	}

	const key = "computer_title"
	if s.HasKey(key) {
		log.Infof(ctx, "Landscape config contains key %q. Its value will be overridden with %s", key, distroName)
		s.DeleteKey(key)
	}

	if _, err := s.NewKey(key, distroName); err != nil {
		return LandscapeConfigure{}, fmt.Errorf("could not create %q key", key)
	}

	w := &bytes.Buffer{}
	if _, err := data.WriteTo(w); err != nil {
		return LandscapeConfigure{}, fmt.Errorf("could not write modified config: %v", err)
	}

	return LandscapeConfigure{
		Config: w.String(),
	}, nil
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
