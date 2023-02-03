package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/i18n"
	"github.com/ubuntu/decorate"
)

const (
	listeningPortFileName = "addr"
)

// GRPCServiceRegisterer is a function that the daemon will call everytime we want to build a new GRPC object.
type GRPCServiceRegisterer func(ctx context.Context) *grpc.Server

// Daemon is a daemon for windows agents with grpc support
type Daemon struct {
	listeningPortFilePath string

	grpcServer *grpc.Server
}

// options are the configurable functional options for the daemon.
type options struct {
	cacheDir string
}

// Option is the function signature we are passing to tweak the daemon creation.
type Option func(*options) error

// WithCacheDir overrides the cache directory used in the daemon.
func WithCacheDir(cachedir string) func(o *options) error {
	return func(o *options) error {
		o.cacheDir = cachedir
		return nil
	}
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer, opts ...Option) (d Daemon, err error) {
	defer decorate.OnError(&err, i18n.G("can't create daemon"))

	log.Debug(ctx, "Building new daemon")

	// Set default options.
	defaultUserCacheDir, err := os.UserCacheDir()
	if err != nil {
		return d, err
	}
	args := options{
		cacheDir: filepath.Join(defaultUserCacheDir, consts.CacheBaseDirectory),
	}

	// Apply given options.
	for _, o := range opts {
		if err := o(&args); err != nil {
			return d, err
		}
	}

	// Create our cache directory if needed
	if err := os.MkdirAll(args.cacheDir, 0750); err != nil {
		return d, err
	}
	listeningPortFilePath := filepath.Join(args.cacheDir, listeningPortFileName)
	log.Debugf(ctx, "Daemon port file path: %s", listeningPortFilePath)

	return Daemon{
		listeningPortFilePath: listeningPortFilePath,
		grpcServer:            registerGRPCServices(ctx),
	}, nil
}

// Serve listens on a tcp socket and starts serving GRPC requests on it.
// Before serving, it writes a file on disk on which port it's listening on for client
// to be able to reach our server.
// This file is removed once the server stops listening.
func (d Daemon) Serve(ctx context.Context) (err error) {
	defer decorate.OnError(&err, i18n.G("error while serving"))

	log.Debug(ctx, "Starting to serve requests")

	lis, err := net.Listen("tcp", "")
	if err != nil {
		return err
	}

	addr := lis.Addr().String()

	// Write a file on disk to signal selected ports to clients.
	// We write it here to signal error when calling service.Start().
	if err := os.WriteFile(d.listeningPortFilePath, []byte(addr), 0640); err != nil {
		return err
	}
	defer os.Remove(d.listeningPortFilePath)

	log.Infof(ctx, "Serving GRPC requests on %v", addr)

	return d.grpcServer.Serve(lis)
}

// Quit gracefully quits listening loop and stops the grpc server.
// It can drops any existing connexion is force is true.
func (d Daemon) Quit(ctx context.Context, force bool) {
	log.Info(ctx, "Stopping daemon requested.")
	if force {
		d.grpcServer.Stop()
		return
	}

	log.Info(ctx, i18n.G("Wait for active requests to close."))
	d.grpcServer.GracefulStop()
	log.Debug(ctx, i18n.G("All connections have now ended."))
}
