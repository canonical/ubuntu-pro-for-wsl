// Package ui implements the GRPC UI service.
package ui

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

// Config is a provider for the subcription configuration.
type Config interface {
	SetSubscription(ctx context.Context, token string, source config.Source) error
	IsReadOnly() (bool, error)
	Subscription(context.Context) (string, config.Source, error)
	FetchMicrosoftStoreSubscription(context.Context) error
}

// Service it the UI GRPC service implementation.
type Service struct {
	db     *database.DistroDB
	config Config

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, config Config, db *database.DistroDB) (s Service) {
	log.Debug(ctx, "Building new GRPC UI service")

	return Service{
		db:     db,
		config: config,
	}
}

// ApplyProToken handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachInfo) (*agentapi.Empty, error) {
	token := info.GetToken()
	log.Debugf(ctx, "Received token %s", common.Obfuscate(token))

	err := s.config.SetSubscription(ctx, token, config.SourceUser)
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

// GetSubscriptionInfo handles the gRPC call to return the type of subscription.
func (s *Service) GetSubscriptionInfo(ctx context.Context, empty *agentapi.Empty) (*agentapi.SubscriptionInfo, error) {
	info := &agentapi.SubscriptionInfo{}

	immutable, err := s.config.IsReadOnly()
	if err != nil {
		return nil, err
	}

	if immutable {
		info.Immutable = true
	}

	_, source, err := s.config.Subscription(ctx)
	if err != nil {
		return nil, err
	}

	switch source {
	case config.SourceNone:
		info.SubscriptionType = &agentapi.SubscriptionInfo_None{}
	case config.SourceUser:
		info.SubscriptionType = &agentapi.SubscriptionInfo_User{}
	case config.SourceRegistry:
		info.SubscriptionType = &agentapi.SubscriptionInfo_Organization{}
	case config.SourceMicrosoftStore:
		info.SubscriptionType = &agentapi.SubscriptionInfo_MicrosoftStore{}
	default:
		return nil, fmt.Errorf("unrecognized subscription source: %d", source)
	}

	return info, nil
}

// NotifyPurchase handles the client notification of a successful purchase through MS Store.
func (s *Service) NotifyPurchase(ctx context.Context, empty *agentapi.Empty) (*agentapi.SubscriptionInfo, error) {
	fetchErr := s.config.FetchMicrosoftStoreSubscription(ctx)
	info, err := s.GetSubscriptionInfo(ctx, empty)
	err = errors.Join(fetchErr, err)
	if err != nil {
		log.Warningf(ctx, "Subscription purchase notification check failed: %v", err)
	}

	return info, err
}
