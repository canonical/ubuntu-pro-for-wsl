// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"

	agent_api "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common/grpc/interceptorschain"
	"github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logconnections"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/wslinstance"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro"
	"github.com/sirupsen/logrus"
	wsl "github.com/ubuntu/gowsl"
	"google.golang.org/grpc"
)

// Manager is the orchestrator of GRPC API services and business logic.
type Manager struct {
	uiService          ui.Service
	wslInstanceService wslinstance.Service
	landscapeService   *landscape.Service
	registryWatcher    *registrywatcher.Service
	db                 *database.DistroDB
	conf               *config.Config
}

// options are the configurable functional options for the daemon.
type options struct {
	registry registrywatcher.Registry
}

// Option is the function signature we are passing to tweak the daemon creation.
type Option func(*options)

// WithRegistry allows overriding the Windows registry with a different back-end.
func WithRegistry(registry registrywatcher.Registry) func(o *options) {
	return func(o *options) {
		o.registry = registry
	}
}

// New returns a new GRPC services manager.
// It instantiates both ui and wsl instance services.
//
// Once done, Stop must be called to deallocate resources.
func New(ctx context.Context, publicDir, privateDir string, args ...Option) (s Manager, err error) {
	log.Debug(ctx, "Building new GRPC services manager")

	defer func() {
		if err != nil {
			// Clean up half-allocated services
			s.Stop(ctx)
		}
	}()

	// Apply given options.
	var opts options
	for _, f := range args {
		f(&opts)
	}

	// Ugly trick to prevent WSL error 0x80070005 due bad interaction with the Store API.
	// See more in:
	//[Jira](https://warthogs.atlassian.net/browse/UDENG-1810)
	//[GitHub](https://github.com/canonical/ubuntu-pro-for-wsl/pull/438)
	InitWSLAPI()

	s.conf = config.New(ctx, privateDir)

	db, err := database.New(ctx, privateDir, s.conf)
	if err != nil {
		return s, err
	}
	s.db = db

	w := registrywatcher.New(ctx, s.conf, s.db, registrywatcher.WithRegistry(opts.registry))
	s.registryWatcher = &w
	s.registryWatcher.Start()

	if err := ubuntupro.FetchFromMicrosoftStore(ctx, s.conf, s.db); err != nil {
		log.Warningf(ctx, "%v", err)
	}

	s.uiService = ui.New(ctx, s.conf, s.db)

	landscape, err := landscape.New(ctx, s.conf, s.db)
	if err != nil {
		return s, err
	}
	s.landscapeService = landscape

	if err := s.landscapeService.Connect(); err != nil {
		log.Warningf(ctx, err.Error())
	}

	wslInstanceService, err := wslinstance.New(ctx, s.db, s.landscapeService.Controller())
	if err != nil {
		return s, err
	}
	s.wslInstanceService = wslInstanceService

	return s, nil
}

// Stop deallocates resources in the services.
func (m Manager) Stop(ctx context.Context) {
	log.Info(ctx, "Stopping GRPC services manager")

	if m.landscapeService != nil {
		m.landscapeService.Stop(ctx)
	}

	if m.registryWatcher != nil {
		m.registryWatcher.Stop()
	}

	if m.db != nil {
		m.db.Close(ctx)
	}

	if m.conf != nil {
		m.conf.Stop()
	}
}

// RegisterGRPCServices returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (m Manager) RegisterGRPCServices(ctx context.Context) *grpc.Server {
	log.Debug(ctx, "Registering GRPC services")

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
			log.StreamServerInterceptor(logrus.StandardLogger()),
			logconnections.StreamServerInterceptor(),
		)))
	agent_api.RegisterUIServer(grpcServer, &m.uiService)
	agent_api.RegisterWSLInstanceServer(grpcServer, &m.wslInstanceService)

	return grpcServer
}

// InitWSLAPI initializes the GoWSL underlying component to prevent access errors due bad interaction
// with the MS Store API, thus it must be called as early as possible.
func InitWSLAPI() {
	d := wsl.NewDistro(context.Background(), "Whatever")
	_, _ = d.GetConfiguration()
}
