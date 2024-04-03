package landscape

import (
	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
)

// These interfaces exist to limit the coupling between components,
// and to make such coupling explicit.

// serviceData is an internal interface to query read-only data from the Landscape service.
type serviceData interface {
	hasStopped() <-chan struct{}
	config() Config
	database() *database.DistroDB
	hostname() string
	homeDir() string
}

// serviceConn is an internal interface to manage the connection to the Landscape service.
type serviceConn interface {
	isDisabled() bool
	connected() bool
	reconnect()
	connDone() <-chan struct{}
	sendInfo(*landscapeapi.HostAgentInfo) error
}
