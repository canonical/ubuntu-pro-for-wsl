// Package commandservice is the implementation of the wsl instance API.
package commandservice

import (
	"context"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
)

// Service is the object in charge of communicating to the Windows agent.
type Service struct {
	system *system.System
}

// New creates a new Wsl instance Service with the provided system.
func New(s *system.System) Service {
	return Service{
		system: s,
	}
}

// ApplyProToken serves ApplyProToken messages sent by the agent.
func (s Service) ApplyProToken(ctx context.Context, info *agentapi.ProAttachCmd) (err error) {
	if info.GetToken() == "" {
		log.Info(ctx, "ApplyProToken: Received empty token: detaching")
	} else {
		log.Infof(ctx, "ApplyProToken: Received token %q: attaching", common.Obfuscate(info.GetToken()))
	}

	if err := s.system.ProDetach(ctx); err != nil {
		return err
	}

	if info.GetToken() == "" {
		return nil
	}

	if err := s.system.ProAttach(ctx, info.GetToken()); err != nil {
		return err
	}

	return nil
}

// ApplyLandscapeConfig serves LandscapeConfig messages sent by the agent.
func (s Service) ApplyLandscapeConfig(ctx context.Context, msg *agentapi.LandscapeConfigCmd) (err error) {
	conf := msg.GetConfig()
	if conf == "" {
		log.Info(ctx, "ApplyLandscapeConfig: received empty config: disabling")
		if err := s.system.LandscapeDisable(ctx); err != nil {
			return err
		}
		return nil
	}

	uid := msg.GetHostagentUid()

	log.Infof(ctx, "ApplyLandscapeConfig: received config: registering")
	if err := s.system.LandscapeEnable(ctx, conf, uid); err != nil {
		return err
	}

	return nil
}
