package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
)

func init() {
	task.Register[*Ping]()
}

// Ping is a task that exists merely to ensure a connection is alive.
type Ping struct{}

const (
	maxAttemps        = 5
	timeoutPerAttempt = 5 * time.Second
)

// Execute is needed to fulfil Task.
func (t *Ping) Execute(ctx context.Context, client wslserviceapi.WSLClient) (err error) {
	for i := 0; i < maxAttemps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err = func() error {
			ctx, cancel := context.WithTimeout(ctx, timeoutPerAttempt)
			defer cancel()

			_, err := client.Ping(ctx, &wslserviceapi.Empty{})
			return err
		}()

		if err != nil {
			continue
		}

		return nil
	}

	return fmt.Errorf("could not ping distro: %v", err)
}

// String is needed to fulfil Task.
func (t Ping) String() string {
	return "Ping"
}
