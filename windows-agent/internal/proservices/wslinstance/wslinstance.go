// Package wslinstance implements the GRPC WSLInstance service.
package wslinstance

import (
	"context"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agent-api"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
)

// Service is the WSL Instance GRPC service implementation.
type Service struct {
	agent_api.UnimplementedWSLInstanceServer
}

// New returns a new service handling WSL Instance API.
func New(ctx context.Context) (s Service, err error) {
	log.Debug(ctx, "Building new GRPC WSL Instance service")

	return Service{}, nil
}
