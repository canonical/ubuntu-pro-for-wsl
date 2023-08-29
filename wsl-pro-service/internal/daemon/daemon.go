// Package daemon handles the GRPC daemon with systemd support.
package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common/i18n"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	"github.com/coreos/go-systemd/daemon"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Daemon is a grpc daemon with systemd support.
type Daemon struct {
	grpcServer *grpc.Server
	addr       string

	systemdSdNotifier systemdSdNotifier
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
func New(ctx context.Context, agentPortFilePath string, registerGRPCService GRPCServiceRegisterer, s system.System, args ...Option) (d Daemon, err error) {
	defer decorate.OnError(&err, i18n.G("can't create daemon"))

	log.Debug(ctx, "Building new daemon")

	// Set default options.
	opts := options{
		systemdSdNotifier: daemon.SdNotify,
	}

	// Apply given args.
	for _, f := range args {
		f(&opts)
	}

	ctrlStream, err := connectToControlStream(ctx, agentPortFilePath, s)
	if err != nil {
		return d, err
	}

	log.Debugf(ctx, "Connected to control stream")

	addr, err := getAddressToListenTo(ctrlStream)
	if err != nil {
		return d, err
	}

	return Daemon{
		grpcServer:        registerGRPCService(ctx, ctrlStream),
		addr:              addr,
		systemdSdNotifier: opts.systemdSdNotifier,
	}, nil
}

// Serve listens on a tcp socket and starts serving GRPC requests on it.
func (d Daemon) Serve(ctx context.Context) (err error) {
	defer decorate.OnError(&err, i18n.G("error while serving"))

	log.Debug(ctx, "Starting to serve requests")

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp4", d.addr)
	if err != nil {
		return err
	}

	log.Infof(ctx, "Serving GRPC requests on %v", d.addr)

	// Signal to systemd that we are ready.
	if sent, err := d.systemdSdNotifier(false, "READY=1"); err != nil {
		return fmt.Errorf(i18n.G("couldn't send ready notification to systemd: %v"), err)
	} else if sent {
		log.Debug(context.Background(), i18n.G("Ready state sent to systemd"))
	}

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

// connectToControlStream connects to the control stream and initiates communication
// by sending the distro's info.
func connectToControlStream(ctx context.Context, agentPortFilePath string, s system.System) (ctrlStream agentapi.WSLInstance_ConnectedClient, err error) {
	defer decorate.OnError(&err, "could not connect to windows agent via the control stream")

	ctrlAddr, err := getControlStreamAddress(agentPortFilePath, s)
	if err != nil {
		return nil, fmt.Errorf("could not get address: %v", err)
	}

	log.Infof(ctx, "Connecting to control stream at %q", ctrlAddr)
	ctrlConn, err := grpc.DialContext(ctx, ctrlAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not dial: %v", err)
	}

	client := agentapi.NewWSLInstanceClient(ctrlConn)
	ctrlStream, err = client.Connected(ctx)
	if err != nil {
		return ctrlStream, fmt.Errorf("could not connect to GRPC service: %v", err)
	}

	sysinfo, err := s.Info(ctx)
	if err != nil {
		return ctrlStream, fmt.Errorf("could not obtain system info: %v", err)
	}

	if err := ctrlStream.Send(sysinfo); err != nil {
		return ctrlStream, fmt.Errorf("could not send system info: %v", err)
	}

	return ctrlStream, nil
}

func getControlStreamAddress(agentPortFilePath string, s system.System) (string, error) {
	/*
		We parse the the port from the file written by the windows agent.
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

	/*
		We parse the IP address of the Windows host /etc/resolv.conf to know. We cannot use localhost
		because WSL localhost forwarding only works for for Linux-side servers.

		# This file was automatically generated by WSL. To stop automatic generation of this file, add the following entry to /etc/wsl.conf:
		# [network]
		# generateResolvConf = false
		nameserver 172.22.16.1
	*/

	r, err := os.Open(s.Path("/etc/resolv.conf"))
	if err != nil {
		return "", err
	}
	defer r.Close()

	var winIP string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var found bool
		suffix, found := strings.CutPrefix(sc.Text(), "nameserver")
		if !found {
			continue
		}
		winIP = strings.TrimSpace(suffix)
		break
	}

	if err := sc.Err(); err != nil {
		return "", err
	}

	if winIP == "" {
		return "", errors.New("could not parse '/etc/resolv.conf': did not find line matching 'nameserver <IP>'")
	}

	return fmt.Sprintf("%s:%s", winIP, port), nil
}

// getAddressToListenTo returns the address where the daemon must listen to.
func getAddressToListenTo(ctrlStream agentapi.WSLInstance_ConnectedClient) (addr string, err error) {
	msg, err := ctrlStream.Recv()
	if err != nil {
		return "", err
	}

	if msg.Port == 0 {
		return "", errors.New("could not get address to serve on: received invalid port :0 from server")
	}

	return fmt.Sprintf("localhost:%d", msg.Port), nil
}
