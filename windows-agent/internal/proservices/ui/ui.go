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
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro/contracts"
	"github.com/ubuntu/decorate"
)

// Config is a provider for the subscription configuration.
type Config interface {
	SetUserSubscription(ctx context.Context, token string) error
	SetStoreSubscription(ctx context.Context, token string) error
	Subscription() (string, config.Source, error)
	SetUserLandscapeConfig(ctx context.Context, token string) error
	LandscapeClientConfig() (string, config.Source, error)
}

// Service it the UI GRPC service implementation.
type Service struct {
	db     *database.DistroDB
	config Config

	// contractsArgs allows for overriding the contract server's behaviour.
	contractsArgs []contracts.Option

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, config Config, db *database.DistroDB, args ...contracts.Option) (s Service) {
	log.Debug(ctx, "Building gRPC UI service")

	return Service{
		db:            db,
		config:        config,
		contractsArgs: args,
	}
}

// ApplyProToken handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachInfo) (_ *agentapi.SubscriptionInfo, err error) {
	defer decorate.LogOnError(err)
	defer decorate.OnError(&err, "UI service: ApplyProToken")

	token := info.GetToken()
	log.Infof(ctx, "UI service: received token %s", common.Obfuscate(token))

	if err := s.config.SetUserSubscription(ctx, token); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("some distros could not pro-attach: %v", err)
	}

	subs, err := s.getSubscriptionSource()
	if err != nil {
		return subs, fmt.Errorf("could not assemble response: %v", err)
	}

	log.Debugf(ctx, "UI service: responding ApplyProToken with following info: %v", subs)
	return subs, nil
}

// ApplyLandscapeConfig handles the gRPC call to set landscape configuration.
func (s *Service) ApplyLandscapeConfig(ctx context.Context, landscapeConfig *agentapi.LandscapeConfig) (*agentapi.Empty, error) {
	c := landscapeConfig.GetConfig()

	err := s.config.SetUserLandscapeConfig(ctx, c)
	if err != nil {
		return nil, err
	}

	return &agentapi.Empty{}, nil
}

// Ping replies a keep-alive request.
func (s *Service) Ping(ctx context.Context, request *agentapi.Empty) (*agentapi.Empty, error) {
	log.Info(ctx, "UI service: received Ping")
	return request, nil
}

// GetConfigSources handles the gRPC call to return the type of subscription and Landscape config sources.
func (s *Service) GetConfigSources(ctx context.Context, empty *agentapi.Empty) (*agentapi.ConfigSources, error) {
	log.Info(ctx, "UI service: received GetConfigSources message")

	subs, err := s.getSubscriptionSource()
	if err != nil {
		err = fmt.Errorf("UI service: GetConfigSources: %v", err)
		log.Warningf(ctx, "%v", err)
		return nil, err
	}

	landscape, err := s.getLandscapeConfigSource()
	if err != nil {
		err = fmt.Errorf("UI service: GetConfigSources: %v", err)
		log.Warningf(ctx, "%v", err)
		return nil, err
	}

	src := &agentapi.ConfigSources{
		LandscapeSource: landscape,
		ProSubscription: subs,
	}
	log.Debugf(ctx, "UI service: responding GetConfigSources with %v", src)
	return src, nil
}

func (s *Service) getSubscriptionSource() (*agentapi.SubscriptionInfo, error) {
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

func (s *Service) getLandscapeConfigSource() (*agentapi.LandscapeSource, error) {
	src := &agentapi.LandscapeSource{}

	_, source, err := s.config.LandscapeClientConfig()
	if err != nil {
		return nil, err
	}

	switch source {
	case config.SourceNone:
		src.LandscapeSourceType = &agentapi.LandscapeSource_None{}
	case config.SourceUser:
		src.LandscapeSourceType = &agentapi.LandscapeSource_User{}
	case config.SourceRegistry:
		src.LandscapeSourceType = &agentapi.LandscapeSource_Organization{}
	default:
		return nil, fmt.Errorf("unrecognized Landscape source: %d", source)
	}

	return src, nil
}

// NotifyPurchase handles the client notification of a successful purchase through MS Store.
func (s *Service) NotifyPurchase(ctx context.Context, empty *agentapi.Empty) (info *agentapi.SubscriptionInfo, errs error) {
	log.Info(ctx, "UI service: received NotifyPurchase message")

	if err := ubuntupro.FetchFromMicrosoftStore(ctx, s.config, s.db, s.contractsArgs...); err != nil {
		log.Warningf(ctx, "UI service: NotifyPurchase: %v", err)
		errs = errors.Join(errs, err)
	}

	info, err := s.getSubscriptionSource()
	if err != nil {
		log.Warningf(ctx, "UI service: NotifyPurchase: %v", err)
		errs = errors.Join(errs, err)
	}

	log.Debugf(ctx, "UI service: responding NotifyPurchase with info: %v", info)
	return info, errs
}
