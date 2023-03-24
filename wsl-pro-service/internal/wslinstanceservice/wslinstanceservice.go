// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
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

// ProAttach serves ProAttach messages sent by the agent.
func (s *Service) ProAttach(ctx context.Context, info *wslserviceapi.AttachInfo) (*wslserviceapi.Empty, error) {
	log.Infof(ctx, "Received ProAttach call with token %q", info.Token)

	attached, err := s.system.ProStatus(ctx)
	if err != nil {
		// TODO: middleware to print errors from task
		log.Errorf(ctx, "Error in ProAttach: ProStatus: %v", err)
		return nil, err
	}

	if attached {
		if err := s.system.ProDetach(ctx); err != nil {
			log.Errorf(ctx, "Error in ProAttach: detachPro: %v", err)
			return nil, err
		}
	}

	err = s.system.ProAttach(ctx, info.Token)
	if err != nil {
		log.Errorf(ctx, "Error in ProAttach: attachPro:: %v", err)
		return nil, err
	}

	log.Debugf(ctx, "ProAttach call: pro attachment complete, sending back result")

	// Check the status again
	sysinfo, err := s.system.Info(ctx)
	if err != nil {
		log.Warning(ctx, "Could not gather system info, skipping send-back to the control stream")
		return nil, nil
	}

	if err := s.ctrlStream.Send(sysinfo); err != nil {
		log.Errorf(ctx, "Error in ProAttach: Send:: %v", err)
		return nil, err
	}

	log.Debugf(ctx, "ProAttach call: finished successfully")
	return &wslserviceapi.Empty{}, nil
}
