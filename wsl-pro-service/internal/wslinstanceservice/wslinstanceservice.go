// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
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
	system systeminfo.System
}

// New creates a new Wsl instance Service with the provided system.
func New(system systeminfo.System) *Service {
	return &Service{
		system: system,
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

// ProServiceEnablement serves ProServiceEnablement messages sent by the agent.
func (s *Service) ProServiceEnablement(ctx context.Context, service *wslserviceapi.ProService) (empty *wslserviceapi.Empty, err error) {
	defer func() {
		// Regardless of success or failure, we send back an updated system info
		if e := s.sendInfo(ctx); e != nil {
			log.Warningf(ctx, "Error in ApplyProToken: %v", e)
			err = errors.Join(err, e)
		}
	}()

	err = s.system.ProEnablement(ctx, service.GetService(), service.GetEnable())
	if err != nil {
		return &wslserviceapi.Empty{}, err
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
