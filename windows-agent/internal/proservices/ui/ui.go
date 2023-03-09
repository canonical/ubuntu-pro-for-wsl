// Package ui implements the GRPC UI service.
package ui

import (
	"context"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
)

// Service it the UI GRPC service implementation.
type Service struct {
	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context) (s Service, err error) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{}, nil
}
