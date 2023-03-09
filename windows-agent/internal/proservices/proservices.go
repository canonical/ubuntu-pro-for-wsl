// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialTasks"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/wslinstance"
	"google.golang.org/grpc"

	// Importing tasks so they are registered and initialTasks can load them.
	// TODO: as soon as we use any task anywhere in the windows agent, this will no longer be necessary.
	_ "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

// Manager is the orchestrator of GRPC API services and business logic.
type Manager struct {
	uiService          ui.Service
	wslInstanceService wslinstance.Service
}

// options are the configurable functional options for the daemon.
type options struct {
	cacheDir string
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

// New returns a new GRPC services manager.
// It instantiates both ui and wsl instance services.
func New(ctx context.Context, args ...Option) (s Manager, err error) {
	log.Debug(ctx, "Building new GRPC services manager")

	// Set default options.
	home := os.Getenv("LocalAppData")
	if home == "" {
		return s, errors.New("Could not read env variable LocalAppData")
	}

	opts := options{
		cacheDir: filepath.Join(home, common.LocalAppDataDir),
	}

	// Apply given options.
	for _, f := range args {
		f(&opts)
	}

	log.Debugf(ctx, "Manager service cache directory: %s", opts.cacheDir)

	if err := os.MkdirAll(opts.cacheDir, 0750); err != nil {
		return s, err
	}

	initTasks, err := initialTasks.New(opts.cacheDir)
	if err != nil {
		return s, err
	}

	db, err := database.New(opts.cacheDir, initTasks)
	if err != nil {
		return s, err
	}

	uiService, err := ui.New(ctx)
	if err != nil {
		return s, err
	}
	wslInstanceService, err := wslinstance.New(ctx, db)
	if err != nil {
		return s, err
	}
	return Manager{
		uiService:          uiService,
		wslInstanceService: wslInstanceService,
	}, nil
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
