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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	db                *database.DistroDB
	config            Config
	landscapeListener chan error

	// contractsArgs allows for overriding the contract server's behaviour.
	contractsArgs []contracts.Option

	agentapi.UnimplementedUIServer
}

// New returns a new service handling the UI API.
func New(ctx context.Context, config Config, db *database.DistroDB, args ...contracts.Option) (s *Service) {
	log.Debug(ctx, "Building gRPC UI service")

	return &Service{
		db:                db,
		config:            config,
		contractsArgs:     args,
		landscapeListener: make(chan error, 1),
	}
}

// Stop deallocates the resources.
func (s *Service) Stop() {
	if s.landscapeListener != nil {
		close(s.landscapeListener)
		s.landscapeListener = nil
	}
}

// drainLandscapeListener drains the landscapeListener channel, dropping any previously unread notifications.
// It returns false if the channel is already closed or and the context is cancelled.
func (s *Service) drainLandscapeListener(ctx context.Context) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case err, ok := <-s.landscapeListener:
			if !ok {
				log.Debug(ctx, "UI service: landscapeListener channel already closed")
				return false
			}
			log.Debugf(ctx, "UI service: dropping unread notification: %v", err)
		default:
			return true
		}
	}
}

// LandscapeConnectionListener is a callback passed to the Landscape service that allows the UI to be notified of
// connection errors.
// This ensures delivery of the last notification from the Landscape service connectivity state, we need that
// because the Landscape service may send events not caused (thus not expected) by the UI service and the contract
// expects this to be a non-blocking callback.
func (s *Service) LandscapeConnectionListener(ctx context.Context, err error) {
	// Drain the channel to prevent blocking on write.
	if !s.drainLandscapeListener(ctx) {
		return
	}

	select {
	case s.landscapeListener <- err:
	case ctxErr := <-ctx.Done():
		log.Warningf(ctx, "UI service: When notifying about Landscape connection: %v", ctxErr)
	}
}

// ApplyProToken handles the gRPC call to pro attach all distros using a token provided by the GUI.
func (s *Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachInfo) (_ *agentapi.SubscriptionInfo, err error) {
	defer decorate.LogOnError(&err)
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
func (s *Service) ApplyLandscapeConfig(ctx context.Context, landscapeConfig *agentapi.LandscapeConfig) (*agentapi.LandscapeSource, error) {
	c := landscapeConfig.GetConfig()

	// Make sure to drain the channel to prevent notifications unrelated to this request.
	s.drainLandscapeListener(ctx)

	err := s.config.SetUserLandscapeConfig(ctx, c)
	if errors.Is(err, config.ErrUserConfigIsNotNew) {
		// The GUI uses gRPC status codes to present meaningful localized error messages.
		return nil, status.Error(codes.AlreadyExists, "user config is not new")
	} else if err != nil {
		return nil, err
	}

	// Blocks until the Landscape service receives an interesting response from the server,
	// thus preventing premature response to the UI and properly propagates any errors.
	if err = <-s.landscapeListener; err != nil {
		log.Warningf(ctx, "UI service: ApplyLandscapeConfig: %v", err)
		return nil, err
	}

	landscape, err := s.getLandscapeConfigSource()
	if err != nil {
		err = fmt.Errorf("UI service: ApplyLandscapeConfig: %v", err)
		log.Warningf(ctx, "%v", err)
		return nil, err
	}
	return landscape, nil
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
