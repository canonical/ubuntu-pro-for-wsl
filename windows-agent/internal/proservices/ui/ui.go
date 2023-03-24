// Package ui implements the GRPC UI service.
package ui

import (
	"context"
	"strings"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialtasks"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

// Service it the UI GRPC service implementation.
type Service struct {
	db           *database.DistroDB
	initialTasks *initialtasks.InitialTasks

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, db *database.DistroDB, initialTasks *initialtasks.InitialTasks) (s Service) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{
		db:           db,
		initialTasks: initialTasks,
	}
}

func obfuscate(contents string) string {
	const endsToReveal = 2
	asterisksLength := len(contents) - 2*endsToReveal
	if asterisksLength < 1 {
		return strings.Repeat("*", len(contents))
	}

	return contents[0:endsToReveal] + strings.Repeat("*", asterisksLength) + contents[asterisksLength+endsToReveal:]
}

// ProAttach handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ProAttach(ctx context.Context, info *agentapi.AttachInfo) (*agentapi.Empty, error) {
	token := info.Token
	log.Debugf(ctx, "Received token %s", obfuscate(token))

	task := tasks.AttachPro{Token: token}
	if err := s.initialTasks.Add(ctx, task); err != nil {
		return nil, err
	}

	distros := s.db.GetAll()
	for _, d := range distros {
		if err := d.SubmitTasks(task); err != nil {
			return nil, err
		}
	}

	return &agentapi.Empty{}, nil
}

// Ping replies a keep-alive request.
func (s *Service) Ping(ctx context.Context, request *agentapi.Empty) (*agentapi.Empty, error) {
	return request, nil
}
