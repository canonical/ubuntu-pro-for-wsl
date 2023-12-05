package landscape

import (
	"context"
	"errors"
	"fmt"
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
	if connected := c.tryReconnect(ctx); !connected {
		return errors.New("could not connect to Landscape")
	}

	conn, release := c.connection()
	defer release()

	info, err := newHostAgentInfo(conn.ctx, c)
	if err != nil {
		return fmt.Errorf("could not assemble message: %v", err)
	}

	return conn.sendUpdatedInfo(info)
}

// tryReconnect sends a "please, connect" signal to the Landscape client and blocks until
// this connection is established, or until the context is canceled. Returns true if the
// connection was successfully established.
func (c Controller) tryReconnect(ctx context.Context) bool {
	if connected(c) {
		// Fast path: connection already exists
		return true
	}

	select {
	case <-ctx.Done():
		return false
	case <-c.hasStopped():
		return false
	case <-c.signalRetryConnection():
	}

	// Petition to reconnect went through, we now wait until it completes
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if connected(c) {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-c.hasStopped():
			return false
		case <-ticker.C:
		}
	}
}
