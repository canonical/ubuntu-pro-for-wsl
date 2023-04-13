// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"
	"errors"
	"fmt"

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
func (s *Service) ProAttach(ctx context.Context, info *wslserviceapi.AttachInfo) (empty *wslserviceapi.Empty, err error) {
	defer func() {
		// Regardless of success or failure, we send back an updated system info
		if e := s.sendInfo(ctx); e != nil {
			log.Warningf(ctx, "Error in ProAttach: %v", e)
			err = errors.Join(err, e)
		}
	}()

	if info.Token == "" {
		log.Info(ctx, "ProAttach: Received empty token: detaching")
	} else {
		log.Infof(ctx, "ProAttach: Received token %q: attaching", info.Token)
	}

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

	if info.Token == "" {
		return &wslserviceapi.Empty{}, nil
	}

	if err := s.system.ProAttach(ctx, info.Token); err != nil {
		log.Errorf(ctx, "Error in ProAttach: attachPro:: %v", err)
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
