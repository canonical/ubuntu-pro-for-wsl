// Package daemon is handling the TCP connection and connecting a GRPC service to it.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
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

	registerer GRPCServiceRegisterer
	grpcServer *grpc.Server

	netSubs *NetWatcher
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer, addrDir string) *Daemon {
	log.Debug(ctx, "Building new daemon")

	listeningPortFilePath := filepath.Join(addrDir, common.ListeningPortFileName)

	return &Daemon{
		listeningPortFilePath: listeningPortFilePath,
		registerer:            registerGRPCServices,
		quit:                  make(chan quitRequest, 1),
		serving:               make(chan struct{}),
		stopped:               make(chan struct{}, 1),
	}
}

type options struct {
	wslCmd                []string
	wslCmdEnv             []string
	getAdaptersAddresses  getAdaptersAddressesFunc
	netMonitoringProvider netmonitoring.DevicesAPIProvider
}

var defaultOptions = options{
	wslCmd:                []string{"wsl.exe"},
	getAdaptersAddresses:  getWindowsAdaptersAddresses,
	netMonitoringProvider: netmonitoring.DefaultAPIProvider,
}

// Option represents an optional function to override getWslIP default values.
type Option func(*options)

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

	opts := defaultOptions
	for _, opt := range args {
		opt(&opts)
	}

	for {
		err := d.tryServingOnce(ctx, opts)
		if errors.Is(err, errRestartDaemon) {
			continue
		}
		return err
	}
}

var errRestartDaemon = errors.New("Daemon: Restart requested")

// Calls d.serve once and handles the possible outcomes of it, returning the error sent via the d.err channel
// plus a true value if it should be restarted. When this function returns, the daemon is no longer serving.
func (d *Daemon) tryServingOnce(ctx context.Context, opts options) error {
	defer func() {
		// let the world know we're currently stopped (probably not in definitive)
		if err := os.Remove(d.listeningPortFilePath); err != nil {
			log.Warningf(ctx, "Daemon: could not remove address file: %v", err)
		}
		d.stopped <- struct{}{}
	}()

	// Try to start serving. This is non-blocking and always returns a readable channel.
	errCh := d.serve(ctx, opts)

	// We now have one serving goroutine.
	// All code paths below must join on errCh to ensure the serving goroutine won't be left detached.
	var quitReq quitRequest
	select {
	case <-ctx.Done():
		// Forceful stop to ensure the goroutine won't leak.
		d.stop(context.Background(), true)
		return errors.Join(ctx.Err(), <-errCh)
	case err := <-errCh:
		return err
	case quitReq = <-d.quit:
		// proceed.
	}

	switch quitReq {
	case quitGraceful:
		d.stop(ctx, false)
		return <-errCh

	case quitForce:
		d.stop(ctx, true)
		return <-errCh

	case restart:
		log.Warning(ctx, "Daemon: Restarting.")
		d.stop(ctx, false)
		// Prevents silently dropping unrelated errors that may have ended the serving goroutine while we handle restarting.
		if err := <-errCh; err != nil {
			log.Debugf(ctx, "Daemon: %v", err)
		}
	}
	// Should restart.
	return errRestartDaemon
}

// cleanup releases all resources held by the daemon, rendering it unusable.
func (d *Daemon) cleanup() {
	defer close(d.stopped)
	d.grpcServer = nil

	if d.netSubs == nil {
		return
	}
	err := d.netSubs.Stop()
	if err != nil {
		log.Errorf(context.Background(), "Daemon: stopping network watcher: %v", err)
	}
	d.netSubs = nil
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

// restart requests the running daemon to restart after completing the RPCs in flight.
// This method returns as soon as the daemon stops serving.
func (d *Daemon) restart(ctx context.Context) {
	select {
	case <-d.serving:
		// proceeds.
	default:
		log.Warning(ctx, "Restart called before Serve.")
		return
	}

	select {
	case <-ctx.Done():
		log.Warning(ctx, "Restart daemon requested meanwhile context was canceled.")
		return

	case d.quit <- restart:
		<-d.stopped
	}
}

type quitRequest int

const (
	quitGraceful quitRequest = iota
	quitForce
	restart
)

// serve implements the actual serving of the daemon, creating a new gRPC server and listening
// on a new goroutine that reports its running status via the returned error channel.
func (d *Daemon) serve(ctx context.Context, opts options) <-chan error {
	log.Debug(ctx, "Daemon: starting to serve requests")

	var lis net.Listener
	wslNetAvailable := true

	// Setting up the listener.
	err := func() (err error) {
		//nolint:govet // i18n depends on strings being acquired at runtime.
		defer decorate.OnError(&err, i18n.G("Daemon: error while serving"))

		wslNetAvailable = true
		wslIP, err := getWslIP(ctx, opts)
		if err != nil {
			wslNetAvailable = false
			wslIP = net.IPv4(127, 0, 0, 1)

			log.Warningf(ctx, "Daemon: could not get the WSL adapter IP: %v. Starting network monitoring", err)
			n, err := subscribe(ctx, func(added []string) bool {
				for _, adapter := range added {
					if strings.Contains(adapter, "(WSL") {
						log.Warningf(ctx, "Daemon: new adapter detected: %s", adapter)
						d.restart(ctx)
						return false
					}
				}

				// Not found yet, let's keep monitoring.
				return true
			}, opts)

			if err != nil {
				log.Errorf(ctx, "Daemon: could not start network monitoring: %v", err)
				// should we return (and not proceed with serving) instead?
			} else {
				d.netSubs = n
			}
		}

		var cfg net.ListenConfig
		lis, err = cfg.Listen(ctx, "tcp", fmt.Sprintf("%s:0", wslIP))
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
		return nil
	}()

	// We may need to write to the channel before readers know about it.
	errCh := make(chan error, 1)
	if err != nil {
		errCh <- err
		// Since the channel is buffered, readers will find the written error.
		close(errCh)
		return errCh
	}

	d.grpcServer = d.registerer(ctx, wslNetAvailable)

	go func() {
		// If we get here, we're the only writer to this channel, thus we are responsible for closing it.
		defer close(errCh)
		err := d.grpcServer.Serve(lis)
		if err != nil {
			err = fmt.Errorf("gRPC serve error: %v", err)
		}

		errCh <- err
	}()

	return errCh
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
