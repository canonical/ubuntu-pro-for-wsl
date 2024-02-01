// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"

	agent_api "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/wslinstance"
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

	conf := config.New(ctx, privateDir)

	db, err := database.New(ctx, privateDir, conf)
	if err != nil {
		return s, err
	}

	registryWatcher := registrywatcher.New(ctx, conf, db, registrywatcher.WithRegistry(opts.registry))
	registryWatcher.Start()

	if err := conf.FetchMicrosoftStoreSubscription(ctx); err != nil {
		log.Warningf(ctx, "%v", err)
	}

	uiService := ui.New(ctx, conf, db)

	landscape, err := landscape.New(conf, db)
	if err != nil {
		return s, err
	}

	if err := landscape.Connect(ctx); err != nil {
		log.Warningf(ctx, err.Error())
	}

	wslInstanceService, err := wslinstance.New(ctx, db, landscape.Controller())
	if err != nil {
		return s, err
	}

	return Manager{
		uiService:          uiService,
		wslInstanceService: wslInstanceService,
		registryWatcher:    &registryWatcher,
		db:                 db,
		landscapeService:   landscape,
	}, nil
}

// Stop deallocates resources in the services.
func (m Manager) Stop(ctx context.Context) {
	if m.landscapeService != nil {
		m.landscapeService.Stop(ctx)
	}

	if m.registryWatcher != nil {
		m.registryWatcher.Stop()
	}

	if m.db != nil {
		m.db.Close(ctx)
	}
}

// RegisterGRPCServices returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (m Manager) RegisterGRPCServices(ctx context.Context) *grpc.Server {
	log.Debug(ctx, "Registering GRPC services")

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
		/*log.StreamServerInterceptor(logrus.StandardLogger()),
		logconnections.StreamServerInterceptor(),*/
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
