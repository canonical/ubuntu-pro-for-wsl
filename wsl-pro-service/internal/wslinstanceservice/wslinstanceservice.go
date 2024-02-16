// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/interceptorschain"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/logconnections"
	log "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-wsl/wslserviceapi"
	"github.com/sirupsen/logrus"
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
	log.Debug(ctx, "Registering gRPC WSL instance service")
	s.ctrlStream = ctrlStream

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
			log.StreamServerInterceptor(logrus.StandardLogger()),
			logconnections.StreamServerInterceptor(),
		)))

	wslserviceapi.RegisterWSLServer(grpcServer, s)

	return grpcServer
}

// ApplyProToken serves ApplyProToken messages sent by the agent.
func (s *Service) ApplyProToken(ctx context.Context, info *wslserviceapi.ProAttachInfo) (empty *wslserviceapi.Empty, err error) {
	defer decorate.OnError(&err, "WSL service")

	defer func() {
		// Regardless of success or failure, we send back an updated system info
		if e := s.sendInfo(ctx); e != nil {
			log.Warningf(ctx, "ApplyProToken: could not send update via control stream: %v", e)
			err = errors.Join(err, e)
		}
	}()

	if info.GetToken() == "" {
		log.Info(ctx, "ApplyProToken: Received empty token: detaching")
	} else {
		log.Infof(ctx, "ApplyProToken: Received token %q: attaching", common.Obfuscate(info.GetToken()))
	}

	if err := s.system.ProDetach(ctx); err != nil {
		return nil, err
	}

	if info.GetToken() == "" {
		return &wslserviceapi.Empty{}, nil
	}

	if err := s.system.ProAttach(ctx, info.GetToken()); err != nil {
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
		return fmt.Errorf("could not send system info: %v", err)
	}

	return nil
}

// ApplyLandscapeConfig serves LandscapeConfig messages sent by the agent.
func (s *Service) ApplyLandscapeConfig(ctx context.Context, msg *wslserviceapi.LandscapeConfig) (empty *wslserviceapi.Empty, err error) {
	defer decorate.OnError(&err, "WSL service")

	conf := msg.GetConfiguration()
	if conf == "" {
		log.Info(ctx, "ApplyLandscapeConfig: received empty config: disabling")
		if err := s.system.LandscapeDisable(ctx); err != nil {
			return nil, err
		}
		return &wslserviceapi.Empty{}, nil
	}

	uid := msg.GetHostagentUID()

	log.Infof(ctx, "ApplyLandscapeConfig: received config: registering")
	if err := s.system.LandscapeEnable(ctx, conf, uid); err != nil {
		return nil, err
	}

	return &wslserviceapi.Empty{}, nil
}
