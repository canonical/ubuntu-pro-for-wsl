package daemon

import (
	"net"
	"os"

	"google.golang.org/grpc"

	agentAPI "github.com/canonical/ubuntu-pro-for-windows/agent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/decorate"
	"github.com/kardianos/service"
)

// GRPCServerRegisterer is a function that the daemon will call once to register all associated GRPC servers.
type GRPCServerRegisterer func(srv *Daemon) *grpc.Server

// Daemon is a daemon for windows agents with grpc support
type Daemon struct {
	lis                net.Listener
	registerGRPCServer GRPCServerRegisterer

	grpcServer grpc.Server
}

// New returns an new, initialized daemon server that is ready to register GRPC services.
// It hooks up to windows service management handler.
func New(registerGRPCServer GRPCServerRegisterer, grpcPort uint) (d *Daemon, err error) {
	defer decorate.OnError(&err /*i18n.G(*/, "can't create daemon" /*)*/)

	os.File
	// FIXME: To look at: https://learn.microsoft.com/en-us/windows/win32/services/interactive-services

	// If we're not running in interactive mode (CLI), add a hook to the logger
	// so that the service can log to the Windows Event Log.
	if !service.Interactive() {
		logger, err := s.Logger(nil)
		if err != nil {
			return nil, err
		}
		//log.AddHook(ctx, &loghooks.EventLog{Logger: logger})
	}

	/*
		Write a file on disk for the selected port.

		LISTEN SHOULD ONLY BE ON START
		lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", grpcPort))
		if err != nil {
			return nil, fmt.Errorf("can't listen on port %d: %v", grpcPort, err)
		}
	*/

	d := &Daemon{
		lis:                lis,
		registerGRPCServer: registerGRPCServer,
	}

	config := service.Config{
		Name:        "ubuntu-pro-agent",
		DisplayName: "Ubuntu Pro Agent",
		Description: "Monitors and manage Ubuntu WSL on your system",
		Option:      service.KeyValue,
	}
	s, err := service.New(d, &config)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func Run() {
	grpcServer := grpc.NewServer()
	agentAPI.RegisterUIServer(grpcServer)
}
