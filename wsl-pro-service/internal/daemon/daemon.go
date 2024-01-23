// Package daemon handles the GRPC daemon with systemd support.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
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

	// Systemd status management.
	systemdSdNotifier systemdSdNotifier
}

// Status sent to systemd.
const (
	serviceStatusWaiting  = "Not serving: waiting to retry"
	serviceStatusRetrying = "Not serving: retrying"
	serviceStatusServing  = "Serving"
	serviceStatusStopped  = "Stopped"
)

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
	defer func() {
		if err := d.systemdNotifyStatus(d.ctx, serviceStatusStopped); err != nil {
			log.Warningf(d.ctx, "Could not change systemd status: %v", err)
		}
	}()

	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
	}

	var gracefulStopCtx context.Context
	gracefulStopCtx, d.gracefulStop = context.WithCancel(d.ctx)
	defer d.gracefulStop()

	var forceStopCtx context.Context
	forceStopCtx, d.forceStop = context.WithCancel(d.ctx)
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

	if err := d.systemdNotifyReady(d.ctx); err != nil {
		return err
	}

	for {
		err := d.serveOnce(gracefulStopCtx, forceStopCtx)
		if err == nil {
			return nil
		}
		var target controlstream.SystemError
		if errors.As(err, &target) {
			// Irrecoverable errors: broken /etc/resolv.conf, broken pro status, etc
			return err
		}
		log.Errorf(d.ctx, "serve error: %v", err)

		delay = min(delay*growthRate, maxDelay)

		if err := d.systemdNotifyStatus(d.ctx, serviceStatusWaiting); err != nil {
			return err
		}

		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-time.After(delay):
		case <-forceStopCtx.Done():
			return nil
		case <-gracefulStopCtx.Done():
			return nil
		}

		log.Infof(d.ctx, "Retrying connection")
		if err := d.systemdNotifyStatus(d.ctx, serviceStatusRetrying); err != nil {
			return err
		}
	}
}

func (d *Daemon) serveOnce(gracefulStopCtx, forceStopCtx context.Context) error {
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	// Initial setup
	if err := d.ctrlStream.Connect(ctx); err != nil {
		return err
	}
	defer d.ctrlStream.Disconnect()
	log.Infof(ctx, "Connected to control stream")

	server := d.registerService(ctx, d.ctrlStream)
	go handleServerStop(ctx, gracefulStopCtx, forceStopCtx, server)

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
}

func handleServerStop(ctx, gracefulStopCtx, forceStopCtx context.Context, server *grpc.Server) {
	defer server.Stop()

	select {
	case <-ctx.Done():
		return
	case <-forceStopCtx.Done():
		return
	case <-gracefulStopCtx.Done():
		server.GracefulStop()
	}

	// Graceful stop can be overridden by a later forced Stop
	select {
	case <-ctx.Done():
	case <-forceStopCtx.Done():
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

	if err := d.systemdNotifyStatus(d.ctx, serviceStatusServing); err != nil {
		return err
	}

	if err := server.Serve(lis); err != nil {
		return fmt.Errorf("grpc error: %v", err)
	}

	return nil
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

func (d *Daemon) systemdNotifyReady(ctx context.Context) error {
	sent, err := d.systemdSdNotifier(false, "READY=1")
	if err != nil {
		return fmt.Errorf(i18n.G("couldn't send ready notification to systemd: %v"), err)
	}
	if sent {
		log.Debug(ctx, i18n.G("Ready state sent to systemd"))
	}
	return nil
}

func (d *Daemon) systemdNotifyStatus(ctx context.Context, status string) error {
	message := fmt.Sprintf("STATUS=%s", status)
	//                             ^^
	// You may think that this should be %q, but you'd be wrong!
	// Using %q causes systemctl to print
	//     Status: ""Hello world""
	// Somehow systemd knows to escape spaces so using %s is the right thing to do:
	//     Status: "Hello world"

	sent, err := d.systemdSdNotifier(false, message)
	if err != nil {
		return fmt.Errorf("couldn't update status to systemd: %v", err)
	}
	if sent {
		log.Debugf(ctx, "Updated systemd status to %q", status)
	}
	return nil
}
