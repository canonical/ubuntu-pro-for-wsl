// Package ui implements the GRPC UI service.
package ui

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/contracts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/ubuntu/decorate"
)

// Config is a provider for the subcription configuration.
type Config interface {
	SetUserSubscription(token string) error
	Subscription() (string, config.Source, error)
	FetchMicrosoftStoreSubscription(context.Context, ...contracts.Option) error
}

// Service it the UI GRPC service implementation.
type Service struct {
	db     *database.DistroDB
	config Config

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, config Config, db *database.DistroDB) (s Service) {
	log.Debug(ctx, "Building gRPC UI service")

	return Service{
		db:     db,
		config: config,
	}
}

// ApplyProToken handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachInfo) (_ *agentapi.SubscriptionInfo, err error) {
	defer decorate.LogOnError(err)
	defer decorate.OnError(&err, "UI service: ApplyProToken")

	token := info.GetToken()
	log.Infof(ctx, "UI service: received token %s", common.Obfuscate(token))

	if err := s.config.SetUserSubscription(token); err != nil {
		return nil, err
	}

	distros := s.db.GetAll()
	for _, d := range distros {
		err = errors.Join(err, d.SubmitTasks(tasks.ProAttachment{Token: token}))
	}

	if err != nil {
		return nil, fmt.Errorf("some distros could not pro-attach: %v", err)
	}

	subs, err := s.getSubscriptionInfo()
	if err != nil {
		return subs, fmt.Errorf("could not assemble response: %v", err)
	}

	log.Debugf(ctx, "UI service: responding ApplyProToken with info: %v", info)
	return subs, nil
}

// Ping replies a keep-alive request.
func (s *Service) Ping(ctx context.Context, request *agentapi.Empty) (*agentapi.Empty, error) {
	log.Info(ctx, "UI service: received Ping")
	return request, nil
}

// GetSubscriptionInfo handles the gRPC call to return the type of subscription.
func (s *Service) GetSubscriptionInfo(ctx context.Context, empty *agentapi.Empty) (_ *agentapi.SubscriptionInfo, err error) {
	log.Info(ctx, "UI service: received GetSubscriptionInfo message")

	info, err := s.getSubscriptionInfo()
	if err != nil {
		err = fmt.Errorf("UI service: GetSubscriptionInfo: %v", err)
		log.Warningf(ctx, "%v", err)
		return nil, err
	}

	log.Debugf(ctx, "UI service: responding GetSubscriptionInfo with %v", info)
	return info, nil
}

func (s *Service) getSubscriptionInfo() (*agentapi.SubscriptionInfo, error) {
	info := &agentapi.SubscriptionInfo{}

	_, source, err := s.config.Subscription()
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
func (s *Service) NotifyPurchase(ctx context.Context, empty *agentapi.Empty) (info *agentapi.SubscriptionInfo, errs error) {
	log.Info(ctx, "UI service: received NotifyPurchase message")

	if err := s.config.FetchMicrosoftStoreSubscription(ctx); err != nil {
		log.Warningf(ctx, "UI service: NotifyPurchase: %v", err)
		errs = errors.Join(errs, err)
	}

	info, err := s.getSubscriptionInfo()
	if err != nil {
		log.Warningf(ctx, "UI service: NotifyPurchase: %v", err)
		errs = errors.Join(errs, err)
	}

	log.Debugf(ctx, "UI service: responding NotifyPurchase with info: %v", info)
	return info, errs
}
