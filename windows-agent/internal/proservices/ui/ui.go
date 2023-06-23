// Package ui implements the GRPC UI service.
package ui

import (
	"context"
	"errors"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

// Service it the UI GRPC service implementation.
type Service struct {
	db     *database.DistroDB
	config *config.Config

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, config *config.Config, db *database.DistroDB) (s Service) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{
		db:     db,
		config: config,
	}
}

// ApplyProToken handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachInfo) (*agentapi.Empty, error) {
	token := info.Token
	log.Debugf(ctx, "Received token %s", common.Obfuscate(token))

	err := s.config.SetSubscription(ctx, token, config.SubscriptionUser)
	if err != nil {
		return nil, err
	}

	distros := s.db.GetAll()
	for _, d := range distros {
		err = errors.Join(err, d.SubmitTasks(tasks.ProAttachment{Token: token}))
	}

	if err != nil {
		log.Debugf(ctx, "Found errors while submitting the ProAttachment task to existing distros:\n%v", err)
		return nil, err
	}

	return &agentapi.Empty{}, nil
}

// Ping replies a keep-alive request.
func (s *Service) Ping(ctx context.Context, request *agentapi.Empty) (*agentapi.Empty, error) {
	return request, nil
}
