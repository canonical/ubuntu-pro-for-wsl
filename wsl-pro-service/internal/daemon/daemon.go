// Package daemon handles the GRPC daemon with systemd support.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common/i18n"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/controlstream"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	"github.com/coreos/go-systemd/daemon"
	"google.golang.org/grpc"
)

// Daemon is a grpc daemon with systemd support.
type Daemon struct {
	ctrlStream      *controlstream.ControlStream
	registerService GRPCServiceRegisterer

	// ctx and cancel used to stop the currently active service.
	ctx    context.Context
	cancel func()

	// Channels for internal messaging.
	started      atomic.Bool
	running      chan struct{}
	gracefulStop func()
	forceStop    func()

	// Sytemd status management.
	systemdSdNotifier systemdSdNotifier
	systemdReadyOnce  sync.Once
}

type options struct {
	systemdSdNotifier systemdSdNotifier
}

type systemdSdNotifier func(unsetEnvironment bool, state string) (bool, error)

// Option is the function signature used to tweak the daemon creation.
type Option func(*options)

// GRPCServiceRegisterer is a function that the daemon will call everytime we want to build a new GRPC object.
type GRPCServiceRegisterer func(context.Context, wslinstanceservice.ControlStreamClient) *grpc.Server

// New returns an new, initialized daemon server, which handles systemd activation.
// If systemd activation is used, it will override any socket passed here.
func New(ctx context.Context, registerGRPCService GRPCServiceRegisterer, s system.System, args ...Option) (*Daemon, error) {
	log.Debug(ctx, "Building new daemon")

	// Set default options.
	opts := options{
		systemdSdNotifier: daemon.SdNotify,
	}

	// Apply given args.
	for _, f := range args {
		f(&opts)
	}

	ctrlStream, err := controlstream.New(ctx, s)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Daemon{
		registerService:   registerGRPCService,
		systemdSdNotifier: opts.systemdSdNotifier,
		ctrlStream:        &ctrlStream,
		ctx:               ctx,
		cancel:            cancel,
	}, nil
}

// Serve sets up the GRPC server to listen to the address reserved by the
// control stream. If either the server or the connection to the stream
// fail, both server and stream are restarted.
func (d *Daemon) Serve() (err error) {
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
	}

	gracefulStop := make(chan struct{})
	var gracefulStopOnce sync.Once
	d.gracefulStop = func() {
		gracefulStopOnce.Do(func() { close(gracefulStop) })
	}
	defer d.gracefulStop()

	forceStop := make(chan struct{})
	var forceStopOnce sync.Once
	d.forceStop = func() {
		forceStopOnce.Do(func() { close(forceStop) })
	}
	defer d.forceStop()

	d.running = make(chan struct{})
	defer close(d.running)

	d.started.Store(true)

	const (
		minDelay   = 1 * time.Second
		maxDelay   = 5 * time.Minute
		growthRate = 2
	)

	delay := minDelay

	for {
		err := func() error {
			ctx, cancel := context.WithCancel(d.ctx)
			defer cancel()

			// Initial setup
			if err := d.ctrlStream.Connect(ctx); err != nil {
				return err
			}
			defer d.ctrlStream.Disconnect()
			log.Infof(ctx, "Connected to control stream")

			server := d.registerService(ctx, d.ctrlStream)
			go d.handleServerStop(ctx, server, gracefulStop, forceStop)

			// Start serving
			serveDone := make(chan error)
			go func() {
				defer close(serveDone)
				serveDone <- d.serve(ctx, server)
			}()

			// Block until either the service or the control stream stops
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-serveDone:
				if err != nil {
					return fmt.Errorf("WSL Pro Service stopped serving: %v", err)
				}
				return nil
			case <-d.ctrlStream.Done(ctx):
				return errors.New("lost connection to Windows Agent")
			}
		}()

		if err == nil {
			return nil
		}

		var target controlstream.SystemError
		if errors.As(err, &target) {
			// Irrecoverable errors: broken /etc/resolv.conf, broken pro status, etc
			return target
		}

		log.Errorf(d.ctx, "serve error: %v", err)
		delay = min(delay*growthRate, maxDelay)

		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-time.After(delay):
		case <-forceStop:
			return nil
		case <-gracefulStop:
			return nil
		}

		log.Infof(d.ctx, "Retrying connection")
	}
}

func handleServerStop(ctx context.Context, server *grpc.Server, gracefulStop <-chan struct{}, forceStop <-chan struct{}) {
	defer server.Stop()

	select {
	case <-ctx.Done():
		return
	case <-forceStop:
		return
	case <-gracefulStop:
		server.GracefulStop()
	}

	// Graceful stop can be overridden by a later forced Stop
	select {
	case <-ctx.Done():
	case <-forceStop:
	}
}

// serve listens on a tcp socket and starts serving GRPC requests on it.
func (d *Daemon) serve(ctx context.Context, server *grpc.Server) error {
	log.Debug(ctx, "Starting to serve requests")

	address := fmt.Sprintf("localhost:%d", d.ctrlStream.ReservedPort())

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp4", address)
	if err != nil {
		return fmt.Errorf("could not listen: %v", err)
	}

	log.Infof(ctx, "Serving GRPC requests on %v", address)

	var failedSignal bool
	d.systemdReadyOnce.Do(func() {
		sent, err := d.systemdSdNotifier(false, "READY=1")
		if err != nil {
			failedSignal = true
			return
		}
		if sent {
			log.Debug(ctx, i18n.G("Ready state sent to systemd"))
		}
	})

	if failedSignal {
		d.systemdReadyOnce = sync.Once{}
		return fmt.Errorf(i18n.G("couldn't send ready notification to systemd: %v"), err)
	}

	if err := server.Serve(lis); err != nil {
		return fmt.Errorf("grpc error: %v", err)
	}

	return nil
}

func getControlStreamAddress(ctx context.Context, agentPortFilePath string, s system.System) (string, error) {
	windowsHostAddr, err := s.WindowsHostAddress(ctx)
	if err != nil {
		return "", err
	}

	/*
		We parse the port from the file written by the windows agent.
	*/
	addr, err := os.ReadFile(agentPortFilePath)
	if err != nil {
		return "", fmt.Errorf("could not read agent port file %q: %v", agentPortFilePath, err)
	}

	fields := strings.Split(string(addr), ":")
	if len(fields) == 0 {
		// Avoid a panic. As far as I know, there is no way of triggering this,
		// but we may as well protect against it.
		return "", fmt.Errorf("could not extract port out of address %q", addr)
	}
	port := fields[len(fields)-1]

	return fmt.Sprintf("%s:%s", windowsHostAddr, port), nil
}

// Quit gracefully quits listening loop and stops the grpc server.
// It can drop any existing connexion is force is set to true.
func (d *Daemon) Quit(ctx context.Context, force bool) {
	defer d.cancel()

	if !d.started.Load() {
		return
	}

	// Signal
	log.Info(ctx, "Stopping daemon requested.")
	if force {
		d.forceStop()
		<-d.running
		return
	}

	d.gracefulStop()

	log.Info(ctx, i18n.G("Waiting for active requests to close."))
	<-d.running
	log.Debug(ctx, i18n.G("All connections have now ended."))
}
