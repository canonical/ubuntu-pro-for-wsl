package task

import (
	"context"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

const (
	maxAttemps        = 5
	timeoutPerAttempt = 5 * time.Second
)

// Ping is a task that exists merely to ensure a connection is alive.
type Ping struct {
	attempt int
}

// Execute is needed to fulfil Task.
func (t *Ping) Execute(ctx context.Context, client wslserviceapi.WSLClient) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutPerAttempt)
	defer cancel()

	_, err := client.Ping(ctx, &wslserviceapi.Empty{})
	t.attempt++
	return err
}

// String is needed to fulfil Task.
func (t *Ping) String() string {
	return "Ping"
}

// ShouldRetry is needed to fulfil Task.
func (t *Ping) ShouldRetry() bool {
	return t.attempt < maxAttemps
}
