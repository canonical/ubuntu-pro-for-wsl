package distro

import (
	"context"
)

const TaskQueueSize = taskQueueSize

func WithTaskProcessingContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.taskProcessingContext = ctx
		}
	}
}
