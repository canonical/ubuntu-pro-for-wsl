// Package daemon is handling the TCP connection and connecting a GRPC service to it.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	log "github.com/canonical/ubuntu-pro-for-windows/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/common/i18n"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

// GRPCServiceRegisterer is a function that the daemon will call everytime we want to build a new GRPC object.
type GRPCServiceRegisterer func(ctx context.Context) *grpc.Server

// Daemon is a daemon for windows agents with grpc support.
type Daemon struct {
	listeningPortFilePath string

	grpcServer *grpc.Server
}

// options are the configurable functional options for the daemon.
type options struct {
	cacheDir string
}

// Option is the function signature we are passing to tweak the daemon creation.
type Option func(*options)

// WithCacheDir overrides the cache directory used in the daemon.
func WithCacheDir(cachedir string) Option {
	return func(o *options) {
		if cachedir != "" {
			o.cacheDir = cachedir
		}
	}
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer, args ...Option) (d Daemon, err error) {
	defer decorate.OnError(&err, i18n.G("can't create daemon"))

	log.Debug(ctx, "Building new daemon")

	// Apply given args.
	var opts options
	for _, f := range args {
		f(&opts)
	}

	if opts.cacheDir == "" {
		// Set default cache dir.
		localAppData := os.Getenv("LocalAppData")
		if localAppData == "" {
			return d, errors.New("Could not read env variable LocalAppData")
		}

		opts.cacheDir = filepath.Join(localAppData, common.LocalAppDataDir)
	}

	// Create our cache directory if needed
	if err := os.MkdirAll(opts.cacheDir, 0750); err != nil {
		return d, err
	}
	listeningPortFilePath := filepath.Join(opts.cacheDir, common.ListeningPortFileName)
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

	// TODO: get a local port only, please :)
	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", "")
	if err != nil {
		return fmt.Errorf("can't listen: %v", err)
	}

	addr := lis.Addr().String()

	// Write a file on disk to signal selected ports to clients.
	// We write it here to signal error when calling service.Start().
	if err := os.WriteFile(d.listeningPortFilePath+".new", []byte(addr), 0600); err != nil {
		return err
	}
	if err := os.Rename(d.listeningPortFilePath+".new", d.listeningPortFilePath); err != nil {
		return err
	}
	defer os.Remove(d.listeningPortFilePath)

	log.Infof(ctx, "Serving GRPC requests on %v", addr)

	if err := d.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc error: %v", err)
	}
	return nil
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
