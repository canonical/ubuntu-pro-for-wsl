// Package distro contains wrappers around actions concerning WSL instances and
// task processing.
package distro

import (
	"context"

	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"google.golang.org/grpc"
)

// Client returns the client to the WSL task service.
// Client returns nil when no connection is set up.
func (d *Distro) Client() wslserviceapi.WSLClient {
	d.connMu.RLock()
	defer d.connMu.RUnlock()

	if d.conn == nil {
		return nil
	}

	return wslserviceapi.NewWSLClient(d.conn)
}

// SetConnection removes the connection associated with the distro.
func (d *Distro) SetConnection(conn *grpc.ClientConn) {
	d.connMu.Lock()
	defer d.connMu.Unlock()

	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			log.Warningf(context.TODO(), "distro %q: could not close previous grpc connection: %v", d.Name, err)
		}
	}
	d.conn = conn
}
