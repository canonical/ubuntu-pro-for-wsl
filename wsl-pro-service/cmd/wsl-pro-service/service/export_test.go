package service

import "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"

func WithAgentPortFilePath(path string) func(*options) {
	return func(o *options) {
		o.agentPortFilePath = path
	}
}

func WithSystem(system systeminfo.System) func(*options) {
	return func(o *options) {
		o.system = system
	}
}
