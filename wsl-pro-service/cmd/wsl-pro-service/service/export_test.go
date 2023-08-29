package service

import "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"

func WithAgentPortFilePath(path string) func(*options) {
	return func(o *options) {
		o.agentPortFilePath = path
	}
}

func WithSystem(s system.System) func(*options) {
	return func(o *options) {
		o.system = s
	}
}
