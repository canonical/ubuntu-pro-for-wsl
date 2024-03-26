package service

import "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"

func WithSystem(s *system.System) func(*options) {
	return func(o *options) {
		o.system = s
	}
}
