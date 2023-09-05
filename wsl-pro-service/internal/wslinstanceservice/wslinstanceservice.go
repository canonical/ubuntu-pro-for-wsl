// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

// ControlStreamClient is the client to the stream between the Windows Agent and the WSL instance service.
type ControlStreamClient interface {
	Send(*agentapi.DistroInfo) error
}

// Service is the object in charge of communicating to the Windows agent.
type Service struct {
	ctrlStream ControlStreamClient

	wslserviceapi.UnimplementedWSLServer
	system system.System
}

// New creates a new Wsl instance Service with the provided system.
func New(s system.System) *Service {
	return &Service{
		system: s,
	}
}

// RegisterGRPCService returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (s *Service) RegisterGRPCService(ctx context.Context, ctrlStream ControlStreamClient) *grpc.Server {
	log.Debug(ctx, "Registering GRPC WSL instance service")
	s.ctrlStream = ctrlStream

	grpcServer := grpc.NewServer()

	wslserviceapi.RegisterWSLServer(grpcServer, s)

	return grpcServer
}

// ApplyProToken serves ApplyProToken messages sent by the agent.
func (s *Service) ApplyProToken(ctx context.Context, info *wslserviceapi.ProAttachInfo) (empty *wslserviceapi.Empty, err error) {
	defer func() {
		// Regardless of success or failure, we send back an updated system info
		if e := s.sendInfo(ctx); e != nil {
			log.Warningf(ctx, "Error in ApplyProToken: %v", e)
			err = errors.Join(err, e)
		}
	}()

	if info.Token == "" {
		log.Info(ctx, "ApplyProToken: Received empty token: detaching")
	} else {
		log.Infof(ctx, "ApplyProToken: Received token %q: attaching", common.Obfuscate(info.Token))
	}

	if err := s.system.ProDetach(ctx); err != nil {
		log.Errorf(ctx, "Error in ApplyProToken: detachPro: %v", err)
		return nil, err
	}

	if info.Token == "" {
		return &wslserviceapi.Empty{}, nil
	}

	if err := s.system.ProAttach(ctx, info.Token); err != nil {
		log.Errorf(ctx, "Error in ApplyProToken: attachPro:: %v", err)
		return nil, err
	}

	return &wslserviceapi.Empty{}, nil
}

func (s *Service) sendInfo(ctx context.Context) error {
	sysinfo, err := s.system.Info(ctx)
	if err != nil {
		return fmt.Errorf("could not gather system info: %v", err)
	}

	if err := s.ctrlStream.Send(sysinfo); err != nil {
		return fmt.Errorf("could not send back system info: %v", err)
	}

	return nil
}

// ApplyLandscapeConfig serves LandscapeConfig messages sent by the agent.
func (s *Service) ApplyLandscapeConfig(ctx context.Context, msg *wslserviceapi.LandscapeConfig) (empty *wslserviceapi.Empty, err error) {
	defer decorate.OnError(&err, "ApplyLandscapeConfig error")

	conf := msg.GetConfiguration()
	if conf == "" {
		log.Info(ctx, "ApplyLandscapeConfig: Received empty config: disabling")
		if err := s.system.LandscapeDisable(ctx); err != nil {
			return nil, err
		}
		return &wslserviceapi.Empty{}, nil
	}

	log.Infof(ctx, "ApplyLandscapeConfig: Received config: registering")
	if err := s.system.LandscapeEnable(ctx, conf); err != nil {
		return nil, err
	}

	return &wslserviceapi.Empty{}, nil
}
