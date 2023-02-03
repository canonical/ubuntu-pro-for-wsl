// Package ui implements the GRPC UI service.
package ui

import (
	"context"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agent-api"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
)

// Service it the UI GRPC service implementation.
type Service struct {
	agent_api.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context) (s Service, err error) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{}, nil
}
