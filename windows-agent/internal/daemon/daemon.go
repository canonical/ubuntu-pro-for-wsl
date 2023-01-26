package daemon

import (
	"context"
	"net"

	"google.golang.org/grpc"

	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/i18n"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/loghooks"
	"github.com/kardianos/service"
	"github.com/ubuntu/decorate"
)

// GRPCServiceRegisterer is a function that the daemon will call everytime we want to build a new GRPC object.
type GRPCServiceRegisterer func(ctx context.Context) *grpc.Server

// Daemon is a daemon for windows agents with grpc support
type Daemon struct {
	ctx context.Context

	listener   listener
	winService service.Service
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(ctx context.Context, registerGRPCServices GRPCServiceRegisterer) (d Daemon, err error) {
	defer decorate.OnError(&err, i18n.G("can't create daemon"))

	log.Debug(ctx, "Building new daemon")

	// FIXME: To look at: https://learn.microsoft.com/en-us/windows/win32/services/interactive-services

	config := service.Config{
		Name:        "ubuntu-pro-agent",
		DisplayName: "Ubuntu Pro Agent",
		Description: "Monitors and manage Ubuntu WSL on your system",
	}

	listener := listener{
		grpcServer: registerGRPCServices(ctx),
	}

	s, err := service.New(&listener, &config)
	if err != nil {
		return d, err
	}

	// If we're not running in interactive mode (CLI), add a hook to the logger
	// so that the service can log to the Windows Event Log.
	if !service.Interactive() {
		logger, err := s.Logger(nil)
		if err != nil {
			return d, err
		}
		log.AddHook(ctx, &loghooks.EventLog{Logger: logger})
	}

	return Daemon{
		ctx: ctx,

		listener:   listener,
		winService: s,
	}, nil
}

// RunAsService runs as a windows service.
func (d Daemon) RunAsService() (err error) {
	defer decorate.OnError(&err, i18n.G("error while running as service"))

	log.Debug(d.ctx, "Run daemon as a service")

	return d.winService.Run()
}

// Run runs syncrhonously, skipping the windows service management part.
func (d Daemon) Run() (err error) {
	defer decorate.OnError(&err, i18n.G("error while running"))

	log.Debug(d.ctx, "Run daemon synchronously")

	lis, err := d.listener.listen()
	if err != nil {
		return err
	}
	return d.listener.serve(lis)
}

// Quit gracefully quits listening loop and stops the grpc server.
// It can drops any existing connexion is force is true.
func (d Daemon) Quit(force bool) {
	log.Info(d.ctx, "Stopping daemon requested.")
	if force {
		d.listener.grpcServer.Stop()
		return
	}

	log.Info(d.ctx, i18n.G("Wait for active requests to close."))
	d.listener.grpcServer.GracefulStop()
	log.Debug(d.ctx, i18n.G("All connections have now ended."))
}

// listener is the internal object which actually deal with socket/GRPC and implements the windows service manager API.
type listener struct {
	ctx context.Context

	grpcServer *grpc.Server
	errs       chan error
}

// Start will start listening and server GRPC requests from the windows service manager.
func (l *listener) Start(s service.Service) (err error) {
	defer decorate.OnError(&err, i18n.G("error while starting service"))

	lis, err := l.listen()
	if err != nil {
		return err
	}

	l.errs = make(chan error)
	go func() {
		l.errs <- l.serve(lis)
	}()

	return nil
}

// Stop will stop server GRPC requests from the windows service manager.
func (l *listener) Stop(s service.Service) (err error) {
	defer decorate.OnError(&err, i18n.G("error while stopping service"))

	l.grpcServer.GracefulStop()

	// Once we are done, return any error from the GRPC server
	return <-l.errs
}

// listen returns a free tcp socket to listen on.
func (l listener) listen() (lis net.Listener, err error) {
	defer decorate.OnError(&err, i18n.G("can't listen"))

	lis, err = net.Listen("tcp", "")
	if err != nil {
		return nil, err
	}

	return lis, nil
}

// serve serves the grpc server on the listener.
// It writes before a file on disk on which port it’s listening on for client.
// This file is removed once the server stop listening.
func (l listener) serve(lis net.Listener) (err error) {
	addr := lis.Addr().String()
	defer decorate.OnError(&err, i18n.G("error while serving on %s"), addr)

	log.Infof(l.ctx, "Serving GRPC requests on %v", addr)

	// TODO: Write a file on disk for the selected port.
	return l.grpcServer.Serve(lis)
}
