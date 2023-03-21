package wslinstanceservice

import (
	"context"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
)

// WithProAttach replaces exec.Command.Output with a custom function to pro-attach.
func WithProAttach(attach func(context.Context, string) ([]byte, error)) Option {
	return func(o *options) {
		o.proAttachCmd = attach
	}
}

// WithProDetach replaces exec.Command.Output with a custom function to pro-detach.
func WithProDetach(out string, err error) Option {
	return func(o *options) {
		o.proDetachCmd = func(ctx context.Context) ([]byte, error) {
			return []byte(out), err
		}
	}
}

// WithProStatus replaces systeminfo.ProStatus with a custom function.
func WithProStatus(proStatus bool, err error) Option {
	return func(o *options) {
		o.proStatus = func(ctx context.Context) (bool, error) {
			return proStatus, err
		}
	}
}

// WithGetSystemInfo replaces systeminfo.Get with a custom function.
func WithGetSystemInfo(info *agentapi.DistroInfo, err error) Option {
	return func(o *options) {
		o.getSystemInfo = func() (*agentapi.DistroInfo, error) {
			return info, err
		}
	}
}
