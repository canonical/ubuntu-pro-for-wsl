package landscape

import (
	"context"
	"errors"
	"time"
)

// Controller is a light-weight structure used to send certain instructions to
// the Landscape service.
type Controller struct {
	serviceConn
	serviceData
}

// SendUpdatedInfo sends a message to the Landscape server with updated
// info about the machine and the distros.
func (c Controller) SendUpdatedInfo(ctx context.Context) error {
	if c.isDisabled() {
		return nil
	}

	if connected := c.tryReconnect(ctx); !connected {
		return errors.New("could not connect to Landscape")
	}

	info, err := newHostAgentInfo(ctx, c)
	if err != nil {
		return err
	}

	return c.sendInfo(info)
}

// Reconnect makes Landscape drop its current connection and start a new one.
// Blocks until the new connection is available (or failed).
func (c Controller) Reconnect(ctx context.Context) (succcess bool) {
	if c.isDisabled() {
		return false
	}

	return c.forceReconnect(ctx)
}

// tryReconnect sends a "please, connect" signal to the Landscape client and blocks until
// this connection is established, or until the context is canceled. Returns true if the
// connection was successfully established.
func (c Controller) tryReconnect(ctx context.Context) bool {
	if c.connected() {
		return true
	}

	return c.forceReconnect(ctx)
}

func (c Controller) forceReconnect(ctx context.Context) bool {
	c.reconnect()

	// Wait until disconnection
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-c.hasStopped():
		return false
	case <-c.connDone():
	}

	// Waiting until re-connection
	ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-c.hasStopped():
			return false
		case <-ticker.C:
		}

		if c.connected() {
			return true
		}
	}
}
