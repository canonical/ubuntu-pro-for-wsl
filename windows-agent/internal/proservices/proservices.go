// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	agent_api "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/grpc/interceptorschain"
	"github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logconnections"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/cloudinit"
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
	"google.golang.org/grpc/credentials"
)

// Manager is the orchestrator of GRPC API services and business logic.
type Manager struct {
	uiService          *ui.Service
	wslInstanceService *wslinstance.Service
	landscapeService   *landscape.Service
	registryWatcher    *registrywatcher.Service
	db                 *database.DistroDB

	creds credentials.TransportCredentials
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

	cloudInit, err := cloudinit.New(ctx, conf, publicDir)
	if err != nil {
		return s, err
	}

	db, err := database.New(
		ctx, privateDir,
		func(d string) {
			err = cloudInit.RemoveDistroData(d)
			if err != nil {
				log.Warningf(ctx, "Could not remove leftover distro data: %v", err)
			}
		},
	)
	if err != nil {
		return s, err
	}
	s.db = db

	w := registrywatcher.New(ctx, conf, s.db, registrywatcher.WithRegistry(opts.registry))
	s.registryWatcher = &w

	s.uiService = ui.New(ctx, conf, s.db)

	landscape, err := landscape.New(ctx, conf, s.db, cloudInit, s.uiService.LandscapeConnectionListener)
	if err != nil {
		return s, err
	}
	s.landscapeService = landscape

	s.wslInstanceService = wslinstance.New(ctx, s.db, s.landscapeService.Controller())

	conf.SetUbuntuProNotifier(func(ctx context.Context, token string) {
		ubuntupro.Distribute(ctx, s.db, token)
		landscape.NotifyUbuntuProUpdate(ctx, token)
		cloudInit.Update(ctx)
	})

	conf.SetLandscapeNotifier(func(ctx context.Context, conf, uid string) {
		log.Warning(ctx, "Landscape features are experimental and not enabled by default in this version.")
		landscape.NotifyConfigUpdate(ctx, conf, uid)
		cloudInit.Update(ctx)
	})

	// All notifications have been set up: starting the registry watcher before any services.
	s.registryWatcher.Start()

	if err := ubuntupro.FetchFromMicrosoftStore(ctx, conf, s.db); err != nil {
		log.Warningf(ctx, "%v", err)
	}

	if err := s.landscapeService.Connect(); err != nil {
		log.Warning(ctx, err.Error())
	}

	destDir := filepath.Join(publicDir, common.CertificatesDir)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return s, fmt.Errorf("failed to create certificates directory: %s", err)
	}
	certs, err := newTLSCertificates(destDir)
	if err != nil {
		return s, fmt.Errorf("failed to create certificates: %s", err)
	}

	s.creds = credentials.NewTLS(certs.agentTLSConfig())
	return s, nil
}

// Stop deallocates resources in the services.
func (m Manager) Stop(ctx context.Context) {
	log.Info(ctx, "Stopping GRPC services manager")

	if m.landscapeService != nil {
		m.landscapeService.Stop(ctx)
	}

	if m.uiService != nil {
		m.uiService.Stop()
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
// If WSL network is not available, the WSLInstance service is not registered.
func (m Manager) RegisterGRPCServices(ctx context.Context, isWslNetAvailable bool) *grpc.Server {
	log.Debug(ctx, "Registering GRPC services")

	// This is never nil because grpc.NewServer() never returns nil.
	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
			log.StreamServerInterceptor(logrus.StandardLogger()),
			logconnections.StreamServerInterceptor(),
		)), grpc.Creds(m.creds))
	agent_api.RegisterUIServer(grpcServer, m.uiService)

	if isWslNetAvailable {
		agent_api.RegisterWSLInstanceServer(grpcServer, m.wslInstanceService)
	}
	return grpcServer
}

// InitWSLAPI initializes the GoWSL underlying component to prevent access errors due bad interaction
// with the MS Store API, thus it must be called as early as possible.
func InitWSLAPI() {
	d := wsl.NewDistro(context.Background(), "Whatever")
	_, _ = d.GetConfiguration()
}
