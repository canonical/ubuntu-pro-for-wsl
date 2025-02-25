package service

import "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"

func WithSystem(s *system.System) func(*options) {
	return func(o *options) {
		o.system = s
	}
}

// DaemonConfig is the configuration for the daemon exported for testing purposes only.
type DaemonConfig = daemonConfig

// Config returns the daemonConfig for test purposes.
func (a App) Config() DaemonConfig {
	return a.config
}
