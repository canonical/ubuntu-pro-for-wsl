// Package daemon handles the TCP and Unix socket connections for the gRPC services.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
)

// GRPCServers are the independent listeners served by the daemon.
type GRPCServers = struct {
	UI  *grpc.Server
	WSL *grpc.Server
}

// GRPCServiceRegisterer is a function that the daemon will call every time we want to build new GRPC objects.
type GRPCServiceRegisterer func(ctx context.Context, isWslNetAvailable bool) GRPCServers

// Daemon is a daemon for windows agents with grpc support.
type Daemon struct {
	listeningPortFilePath string
	uiSocketPath          string

	// serving signals that Serve has been called once. This channel is closed when Serve is called.
	serving chan struct{}

	// quit allows other goroutines to signal to stop the daemon while still running. It's intentionally never closed so clients can call Quit() safely.
	quit chan quitRequest

	// stopped lets the Quit() method block the caller until the daemon has stopped serving.
	stopped chan struct{}

	registerer GRPCServiceRegisterer

	netSubs *NetWatcher
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer, addrDir string, privateDirs ...string) *Daemon {
	log.Debug(ctx, "Building new daemon")

	privateDir := addrDir
	if len(privateDirs) > 0 && privateDirs[0] != "" {
		privateDir = privateDirs[0]
	}
	listeningPortFilePath := filepath.Join(addrDir, common.ListeningPortFileName)

	return &Daemon{
		listeningPortFilePath: listeningPortFilePath,
		uiSocketPath:          filepath.Join(privateDir, common.UISocketFileName),
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

// WaitReady blocks until the daemon is ready to serve, i.e. until Serve has been called.
func (d *Daemon) WaitReady() {
	<-d.serving
}

// Option represents an optional function to override getWslIP default values.
type Option func(*options)

// Serve listens on TCP and Unix sockets and starts serving gRPC requests on them.
// Before serving, it writes a file on disk on which port it's listening on for client
// to be able to reach our server.
// This file is removed once the server stops listening.
// The server is automatically restarted if it was stopped by a concurrent call to Restart().
// This method is designed to be called just and only once, when it returns the daemon is no longer useful.
func (d *Daemon) Serve(ctx context.Context, args ...Option) error {
	select {
	case <-d.serving:
		return errors.New("Serve called more than once")
	case <-d.stopped:
		return errors.New("Serve called after Quit")
	default:
		// Proceeds.
	}
	// Once this method leaves the daemon is done forever.
	defer d.cleanup()

	opts := defaultOptions
	for _, opt := range args {
		opt(&opts)
	}

	// let the world know we were requested to serve.
	close(d.serving)

	for {
		err := d.tryServingOnce(ctx, opts)
		if errors.Is(err, errRestartDaemon) {
			continue
		}
		return err
	}
}

var errRestartDaemon = errors.New("Daemon: Restart requested")

// tryServingOnce calls d.serve once and handles the possible outcomes of it, returning the error sent via the d.err channel
// plus a true value if it should be restarted. When this function returns, the daemon is no longer serving.
func (d *Daemon) tryServingOnce(ctx context.Context, opts options) error {
	defer func() {
		// let the world know we're currently stopped (probably not in definitive)
		if err := os.Remove(d.listeningPortFilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warningf(ctx, "Daemon: could not remove address file: %v", err)
		}
		if err := os.Remove(d.uiSocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warningf(ctx, "Daemon: could not remove UI socket: %v", err)
		}
		d.stopped <- struct{}{}
	}()

	// Try to start serving. This is non-blocking and always returns a readable channel.
	errCh, stop := d.serve(ctx, opts)

	// We now have one serving goroutine.
	// All code paths below must join on errCh to ensure the serving goroutine won't be left detached.
	var quitReq quitRequest
	select {
	case <-ctx.Done():
		// Forceful stop to ensure the goroutine won't leak.
		stop(context.Background(), true)
		return errors.Join(ctx.Err(), <-errCh)
	case err := <-errCh:
		return err
	case quitReq = <-d.quit:
		// proceed.
	}

	switch quitReq {
	case quitGraceful:
		stop(ctx, false)
		return <-errCh

	case quitForce:
		stop(ctx, true)
		return <-errCh

	case restart:
		log.Warning(ctx, "Daemon: Restarting.")
		stop(ctx, false)
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

	if d.netSubs == nil {
		return
	}
	if err := d.netSubs.Stop(); err != nil {
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

	// This select binds the time this would block on sending via d.quit (when the channel is full) to the context cancellation.
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
// Call the returned stopCallback to stop the server either gracefully or forcefully.
func (d *Daemon) serve(ctx context.Context, opts options) (<-chan error, stopFunc) {
	log.Debug(ctx, "Daemon: starting to serve requests")

	var tcpLis, uiLis net.Listener
	wslNetAvailable := true

	// Set up the TCP listener used by WSL instances.
	err := func() (err error) {
		defer decorate.OnError(&err, i18n.G("Daemon: error while serving"))

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
				return true
			}, opts)
			if err != nil {
				return fmt.Errorf("Daemon: could not start network monitoring: %v", err)
			}
			d.netSubs = n
		}

		var cfg net.ListenConfig
		tcpLis, err = cfg.Listen(ctx, "tcp", fmt.Sprintf("%s:0", wslIP))
		if err != nil {
			return fmt.Errorf("can't listen on TCP: %v", err)
		}

		// The UI endpoint is deliberately independent of the WSL TCP endpoint.
		// Remove a stale socket left behind by an unclean agent shutdown.
		if removeErr := os.Remove(d.uiSocketPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("can't remove stale UI socket: %v", removeErr)
		}
		uiLis, err = cfg.Listen(ctx, "unix", d.uiSocketPath)
		if err != nil {
			return fmt.Errorf("can't listen on UI socket: %v", err)
		}

		addr := tcpLis.Addr().String()
		if err := os.WriteFile(d.listeningPortFilePath, []byte(addr), 0600); err != nil {
			return err
		}

		log.Debugf(ctx, "Daemon: address file written to %s", d.listeningPortFilePath)
		log.Infof(ctx, "Daemon: serving WSL gRPC requests on %s", addr)
		log.Infof(ctx, "Daemon: serving UI gRPC requests on %s", d.uiSocketPath)
		return nil
	}()

	errCh := make(chan error, 2)
	if err != nil {
		if tcpLis != nil {
			_ = tcpLis.Close()
		}
		if uiLis != nil {
			_ = uiLis.Close()
		}
		errCh <- err
		close(errCh)
		return errCh, func(context.Context, bool) {}
	}

	servers := d.registerer(ctx, wslNetAvailable)
	listeners := []struct {
		server   *grpc.Server
		listener net.Listener
		name     string
	}{{servers.UI, uiLis, "UI"}}
	if servers.WSL != nil {
		listeners = append(listeners, struct {
			server   *grpc.Server
			listener net.Listener
			name     string
		}{servers.WSL, tcpLis, "WSL"})
	} else {
		// There is no WSL service when the WSL network is unavailable. The
		// address file is still written as the startup readiness signal.
		_ = tcpLis.Close()
	}

	var stopAll func(bool)
	var stopOnce sync.Once
	stopAll = func(force bool) {
		stopOnce.Do(func() {
			for _, entry := range listeners {
				if force {
					entry.server.Stop()
				} else {
					entry.server.GracefulStop()
				}
			}
		})
	}

	var wg sync.WaitGroup
	wg.Add(len(listeners))
	for _, entry := range listeners {
		entry := entry
		go func() {
			defer wg.Done()
			serveErr := entry.server.Serve(entry.listener)
			if serveErr != nil {
				serveErr = fmt.Errorf("gRPC %s serve error: %v", entry.name, serveErr)
			}
			errCh <- serveErr
			// If one listener exits unexpectedly, do not leave the other one running.
			if serveErr != nil {
				stopAll(true)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()

	return errCh, func(ctx context.Context, force bool) {
		log.Info(ctx, "Stopping daemon requested.")
		if force {
			stopAll(true)
			return
		}
		log.Info(ctx, i18n.G("Daemon: waiting for active requests to close."))
		stopAll(false)
		log.Debug(ctx, i18n.G("Daemon: all connections have now ended."))
	}
}

type stopFunc func(ctx context.Context, force bool)
