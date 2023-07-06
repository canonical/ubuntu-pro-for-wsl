// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/wslinstance"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

// Manager is the orchestrator of GRPC API services and business logic.
type Manager struct {
	uiService          ui.Service
	wslInstanceService wslinstance.Service
	landscapeService   *landscape.Client
	db                 *database.DistroDB
}

// options are the configurable functional options for the daemon.
type options struct {
	cacheDir string
	registry config.Registry
}

// Option is the function signature we are passing to tweak the daemon creation.
type Option func(*options)

// WithCacheDir overrides the cache directory used in the daemon.
func WithCacheDir(cachedir string) func(o *options) {
	return func(o *options) {
		if cachedir != "" {
			o.cacheDir = cachedir
		}
	}
}

// WithRegistry allows overriding the Windows registry with a different back-end.
func WithRegistry(registry config.Registry) func(o *options) {
	return func(o *options) {
		o.registry = registry
	}
}

// New returns a new GRPC services manager.
// It instantiates both ui and wsl instance services.
//
// Once done, Stop must be called to deallocate resources.
func New(ctx context.Context, args ...Option) (s Manager, err error) {
	log.Debug(ctx, "Building new GRPC services manager")

	// Apply given options.
	var opts options
	for _, f := range args {
		f(&opts)
	}

	if opts.cacheDir == "" {
		// Set default cache dir.
		appData := os.Getenv("LocalAppData")
		if appData == "" {
			return s, errors.New("Could not read env variable LocalAppData")
		}

		opts.cacheDir = filepath.Join(appData, common.LocalAppDataDir)
	}

	log.Debugf(ctx, "Manager service cache directory: %s", opts.cacheDir)

	if err := os.MkdirAll(opts.cacheDir, 0750); err != nil {
		return s, err
	}

	conf := config.New(ctx, opts.cacheDir, config.WithRegistry(opts.registry))

	db, err := database.New(ctx, opts.cacheDir, conf)
	if err != nil {
		return s, err
	}
	defer func() {
		if err != nil {
			db.Close(ctx)
		}
	}()

	uiService := ui.New(ctx, conf, db)

	landscape, err := landscape.NewClient(conf, db, opts.cacheDir)
	if err != nil {
		return s, err
	}

	if err := landscape.Connect(ctx); err != nil {
		log.Warningf(ctx, err.Error())
	}

	wslInstanceService, err := wslinstance.New(ctx, db, landscape)
	if err != nil {
		return s, err
	}

	return Manager{
		uiService:          uiService,
		wslInstanceService: wslInstanceService,
		db:                 db,
		landscapeService:   landscape,
	}, nil
}

// Stop deallocates resources in the services.
func (m Manager) Stop(ctx context.Context) {
	m.landscapeService.Disconnect(ctx)
	m.db.Close(ctx)
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
