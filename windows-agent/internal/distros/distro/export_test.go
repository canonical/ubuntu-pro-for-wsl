package distro

import (
	"context"
)

func WithTaskProcessingContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.taskProcessingContext = ctx
		}
	}
}

// WithNewWorker is an optional parameter for distro.New that allows for overriding
// the worker.New constructor. It is meant for dependency injection.
func WithNewWorker(newWorkerFunc func(context.Context, *Distro, string) (workerInterface, error)) Option {
	return func(o *options) {
		o.newWorkerFunc = newWorkerFunc
	}
}

type Worker = workerInterface

// Identity contains persistent and uniquely identifying information about the distro.
type Identity = identity

// GetIdentity returns a reference to the distro's identity.
//
//nolint:revive // False positive, Identity is exported.
func (d *Distro) GetIdentity() *Identity {
	return &d.identity
}
