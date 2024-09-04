// Package daemon is handling the TCP connection and connecting a GRPC service to it.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

// GRPCServiceRegisterer is a function that the daemon will call everytime we want to build a new GRPC object.
type GRPCServiceRegisterer func(ctx context.Context, isWslNetAvailable bool) *grpc.Server

// Daemon is a daemon for windows agents with grpc support.
type Daemon struct {
	listeningPortFilePath string

	// serving signals that Serve has been called once. This channel is closed when Serve is called.
	serving chan struct{}

	// quit allows other goroutines to signal to stop the daemon while still running. It's intentionally never closed so clients can call Quit() safely.
	quit chan quitRequest

	// stopped lets the Quit() method block the caller until the daemon has stopped serving.
	stopped chan struct{}

	// err is channel through which the current serving goroutine will deliver its exit error, if any.
	// It's intentionally never closed because a writer cannot know up-front it will be the last one.
	err chan error

	registerer GRPCServiceRegisterer
	grpcServer *grpc.Server
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer, addrDir string) *Daemon {
	log.Debug(ctx, "Building new daemon")

	listeningPortFilePath := filepath.Join(addrDir, common.ListeningPortFileName)

	return &Daemon{
		listeningPortFilePath: listeningPortFilePath,
		registerer:            registerGRPCServices,
		err:                   make(chan error, 1),
		quit:                  make(chan quitRequest, 1),
		serving:               make(chan struct{}),
		stopped:               make(chan struct{}, 1),
	}
}

// Serve listens on a tcp socket and starts serving GRPC requests on it.
// Before serving, it writes a file on disk on which port it's listening on for client
// to be able to reach our server.
// This file is removed once the server stops listening.
// The server is automatically restarted if it was stopped by a concurrent call to Restart().
// This method is designed to be called just and only once, when it returns the daemon is no longer useful.
func (d *Daemon) Serve(ctx context.Context, args ...Option) error {
	// Once this method leaves the daemon is done forever.
	defer d.cleanup()

	// let the world know we were requested to serve.
	close(d.serving)

	for {
		retry, err := d.tryServingOnce(ctx, args...)
		if retry {
			continue
		}
		return err
	}
}

// Calls d.serve once and handles the possible outcomes of it, returning the error sent via the d.err channel
// plus a true value if it should be restarted. When this function returns, the daemon is no longer serving.
func (d *Daemon) tryServingOnce(ctx context.Context, args ...Option) (bool, error) {
	defer func() {
		// let the world know we're currently stopped (probably not in definitive)
		_ = os.Remove(d.listeningPortFilePath)
		d.stopped <- struct{}{}
	}()

	// Try to start serving.
	if err := d.serve(ctx, args...); err != nil {
		return false, err
	}

	// We now have one serving goroutine.
	// All code paths below must join on d.err to ensure the serving goroutine won't be left detached.
	var quitReq quitRequest
	select {
	case <-ctx.Done():
		// Forceful stop to ensure the goroutine won't leak.
		d.stop(context.Background(), true)
		return false, errors.Join(ctx.Err(), <-d.err)
	case err := <-d.err:
		return false, err
	case quitReq = <-d.quit:
		// proceed.
	}

	switch quitReq {
	case quitGraceful:
		d.stop(ctx, false)
		return false, <-d.err

	case quitForce:
		d.stop(ctx, true)
		return false, <-d.err
	}
	// Should restart => for now unreachable. Fix coming soon.
	return true, nil
}

// cleanup releases all resources held by the daemon, rendering it unusable.
func (d *Daemon) cleanup() {
	defer close(d.stopped)
	d.grpcServer = nil
}

// Quit gracefully quits listening loop and stops the grpc server.
// It can drop any existing connexion if force is true.
// Although this method is idempotent, once it returns, the daemon is no longer useful.
func (d *Daemon) Quit(ctx context.Context, force bool) {
	select {
	case <-d.serving:
		// proceeds.
	default:
		log.Warning(ctx, "Quit called before Serve.")
		return
	}

	req := quitGraceful
	if force {
		req = quitForce
	}

	select {
	case <-ctx.Done():
		log.Warning(ctx, "Stop daemon requested meanwhile context was canceled.")
		return

	case d.quit <- req:
		<-d.stopped
	}
}

type quitRequest int

const (
	quitGraceful quitRequest = iota
	quitForce
)

// serve implements the actual serving of the daemon, creating a new gRPC server and listening
// on a new goroutine that reports its running status and errors via the daemon channels:
// - d.stopped to let callers of Quit() remain blocked until it exits and
// - d.err to let the caller of Serve() know if it exited with an error.
func (d *Daemon) serve(ctx context.Context, args ...Option) (err error) {
	//nolint:govet // i18n depends on strings being acquired at runtime.
	defer decorate.OnError(&err, i18n.G("Daemon: error while serving"))

	log.Debug(ctx, "Daemon: starting to serve requests")

	wslNetAvailable := true
	wslIP, err := getWslIP(ctx, args...)
	if err != nil {
		log.Warningf(ctx, "could not get the WSL adapter IP: %v", err)
		wslNetAvailable = false
		wslIP = net.IPv4(127, 0, 0, 1)
	}

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", fmt.Sprintf("%s:0", wslIP))
	if err != nil {
		return fmt.Errorf("can't listen: %v", err)
	}

	addr := lis.Addr().String()

	// Write a file on disk to signal selected ports to clients.
	// We write it here to signal error when calling service.Start().
	if err := os.WriteFile(d.listeningPortFilePath, []byte(addr), 0600); err != nil {
		return err
	}

	log.Debugf(ctx, "Daemon: address file written to %s", d.listeningPortFilePath)
	log.Infof(ctx, "Daemon: serving gRPC requests on %s", addr)

	d.grpcServer = d.registerer(ctx, wslNetAvailable)

	go func() {
		err := d.grpcServer.Serve(lis)
		// This is the only place where we write into d.err so it can report this goroutine being done.
		d.err <- wrapf("gRPC serve error: %v", err)
	}()

	return nil
}

// wrapf wraps an error with fmt.Errorf(), returning nil if err is nil.
func wrapf(format string, err error, a ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format, append(a, err)...)
}

// Handles stopping the daemon's gRPC server.
// This must be called by the same goroutine that started the server.
func (d *Daemon) stop(ctx context.Context, force bool) {
	// ... thus no need to check d.grpcServer for nil.
	log.Info(ctx, "Stopping daemon requested.")

	if force {
		d.grpcServer.Stop()
		return
	}

	log.Info(ctx, i18n.G("Daemon: waiting for active requests to close."))
	d.grpcServer.GracefulStop()
	log.Debug(ctx, i18n.G("Daemon: all connections have now ended."))
}
