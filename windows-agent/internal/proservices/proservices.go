package proservices

import (
	"context"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/interceptorschain"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logconnections"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/wslinstance"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Manager struct {
	uiService          ui.Service
	wslInstanceService wslinstance.Service
}

// New returns a new GRPC services manager.
// It instantiates both ui and wsl instance services.
func New(ctx context.Context) (s Manager, err error) {
	log.Debug(ctx, "Building new GRPC services manager")

	uiService, err := ui.New(ctx)
	if err != nil {
		return s, err
	}
	wslInstanceService, err := wslinstance.New(ctx)
	if err != nil {
		return s, err
	}
	return Manager{
		uiService:          uiService,
		wslInstanceService: wslInstanceService,
	}, nil
}

func (m Manager) RegisterGRPCServices(ctx context.Context) *grpc.Server {
	log.Debug(ctx, "Registering GRPC services")

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
			log.StreamServerInterceptor(logrus.StandardLogger()),
			logconnections.StreamServerInterceptor(),
		)))
	agent_api.RegisterUIServer(grpcServer, m.uiService)
	agent_api.RegisterWSLInstanceServer(grpcServer, m.wslInstanceService)

	return grpcServer
}
