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

// Identity contains persistent and uniquely identifying information about the distro.
type Identity = identity

// GetIdentity returns a reference to the distro's identity.
//
//nolint: revive
// False positive, Identity is exported.
func (d *Distro) GetIdentity() *Identity {
	return &d.identity
}
