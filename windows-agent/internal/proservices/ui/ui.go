// Package ui implements the GRPC UI service.
package ui

import (
	"context"
	"errors"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialTasks"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

// Service it the UI GRPC service implementation.
type Service struct {
	db           *database.DistroDB
	initialTasks *initialTasks.InitialTasks

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, db *database.DistroDB, initialTasks *initialTasks.InitialTasks) (s Service) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{
		db:           db,
		initialTasks: initialTasks,
	}
}

// ProAttach handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ProAttach(ctx context.Context, info *agentapi.AttachInfo) (*agentapi.Empty, error) {
	token := info.Token
	log.Debugf(ctx, "Received token %s", token)

	task := tasks.AttachPro{Token: token}
	if err := s.initialTasks.Add(ctx, task); err != nil {
		return nil, err
	}

	// TODO: Replace this by getting all active distros.
	distro, ok := s.db.Get("Ubuntu-Preview")
	if !ok {
		return nil, errors.New("Ubuntu-Preview doesn't exist")
	}
	if err := distro.SubmitTasks(task); err != nil {
		return nil, err
	}
	return nil, nil
}

// Ping replies a keep-alive request.
func (s *Service) Ping(ctx context.Context, request *agentapi.Empty) (*agentapi.Empty, error) {
	return request, nil
}
