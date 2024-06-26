// Package daemon handles the GRPC daemon with systemd support.
package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/streams"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"github.com/coreos/go-systemd/daemon"
	"github.com/sirupsen/logrus"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Daemon is a grpc daemon with systemd support.
type Daemon struct {
	addressPath, certsPath string

	// Interface to the WSL distro
	system *system.System

	// Systemd status management.
	systemdSdNotifier systemdSdNotifier

	// Channels for internal messaging.
	started atomic.Bool
	running chan struct{}

	// This context is used to interrupt any action.
	// It must be the parent of gracefulCtx.
	ctx    context.Context
	cancel context.CancelFunc

	// This context waits until the next blocking Recv to interrupt.
	gracefulCtx    context.Context
	gracefulCancel context.CancelFunc
}

// Status sent to systemd.
const (
	serviceStatusWaiting    = "Not connected: waiting to retry"
	serviceStatusConnecting = "Connecting"
	serviceStatusConnected  = "Connected"
	serviceStatusStopped    = "Stopped"
)

type options struct {
	systemdSdNotifier systemdSdNotifier
}

type systemdSdNotifier func(unsetEnvironment bool, state string) (bool, error)

// Option is the function signature used to tweak the daemon creation.
type Option func(*options)

// New returns an new, initialized daemon server, which handles systemd activation.
// If systemd activation is used, it will override any socket passed here.
func New(ctx context.Context, s *system.System, args ...Option) (*Daemon, error) {
	log.Debug(ctx, "Building new daemon")

	// Set default options.
	opts := options{
		systemdSdNotifier: daemon.SdNotify,
	}

	// Apply given args.
	for _, f := range args {
		f(&opts)
	}

	home, err := s.UserProfileDir(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not find address file: could not find $env:UserProfile: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	gCtx, gCancel := context.WithCancel(ctx)

	return &Daemon{
		systemdSdNotifier: opts.systemdSdNotifier,
		system:            s,
		addressPath:       filepath.Join(home, common.UserProfileDir, common.ListeningPortFileName),
		certsPath:         filepath.Join(home, common.UserProfileDir, common.CertificatesDir),

		ctx:    ctx,
		cancel: cancel,

		gracefulCtx:    gCtx,
		gracefulCancel: gCancel,
	}, nil
}

// Serve serves on the streams, automatically reconnecting when the connection drops.
// Call Quit to deallocate the resources used in Serve.
func (d *Daemon) Serve(service streams.CommandService) error {
	defer d.cancel()
	defer d.systemdNotifyStatus(d.ctx, serviceStatusStopped)

	d.running = make(chan struct{})
	defer close(d.running)

	d.started.Store(true)

	select {
	case <-d.gracefulCtx.Done():
		return errors.New("already quit")
	default:
	}

	// Exponential back-off
	const (
		minWait      = time.Second
		maxWait      = time.Minute
		growthFactor = 2
	)
	wait := 0 * time.Second

	// Signal systemd before dialing for the first time
	// We don't want to delay startup due to a timeout
	err := d.systemdNotifyReady(d.ctx)
	if err != nil {
		return fmt.Errorf("could not notify systemd: %v", err)
	}

	for {
		select {
		case <-d.gracefulCtx.Done():
			return nil
		case <-time.After(wait):
		}

		success, err := func() (success bool, err error) {
			// ctx handles force-quit
			ctx, cancel := context.WithCancel(d.ctx)
			defer cancel()

			log.Info(ctx, "Daemon: connecting to Windows Agent")
			d.systemdNotifyStatus(ctx, serviceStatusConnecting)

			server, err := d.connect(ctx)
			if errors.Is(err, streams.SystemError{}) {
				return false, err
			} else if err != nil {
				log.Warningf(ctx, "Daemon: %v", err)
				return false, nil
			}

			go func() {
				// Handle graceful quit.
				select {
				case <-d.gracefulCtx.Done():
				case <-ctx.Done():
				}
				server.GracefulStop()
			}()

			log.Info(ctx, "Daemon: completed connection to Windows Agent")
			d.systemdNotifyStatus(ctx, serviceStatusConnected)

			t := time.NewTimer(time.Minute)
			defer t.Stop()

			err = server.Serve(service)

			if errors.Is(err, streams.SystemError{}) {
				return false, err
			} else if err != nil {
				log.Warningf(ctx, "Daemon: disconnected from Windows host: %v", err)
			} else {
				log.Warning(ctx, "Daemon: disconnected from Windows host")
			}

			select {
			case <-t.C:
				// Long-lived connection is not a failure
				return true, nil
			default:
				// Connection was short-lived: consider it a failure
				return false, nil
			}
		}()

		if err != nil {
			return err
		}

		if success {
			wait *= 0
			continue
		}

		wait = clamp(minWait, wait*growthFactor, maxWait)
		log.Infof(d.ctx, "Reconnecting to Windows host in %d seconds", int(wait/time.Second))
		d.systemdNotifyStatus(d.ctx, serviceStatusWaiting)
	}
}

// Quit gracefully quits listening loop and stops the grpc server.
// It can drop any existing connection if force is set to true.
func (d *Daemon) Quit(ctx context.Context, force bool) {
	defer d.cancel()

	// Signal
	log.Info(ctx, "Stopping daemon requested.")
	if force {
		d.cancel()
		log.Info(ctx, i18n.G("Stopping active requests."))
	} else {
		d.gracefulCancel()
		log.Info(ctx, i18n.G("Waiting for active requests to close."))
	}

	if !d.started.Load() {
		return
	}

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
		if err := os.Unsetenv("NOTIFY_SOCKET"); err != nil {
			log.Warningf(ctx, "couldn't unset NOTIFY_SOCKET for subprocesses: %v", err)
		}
	}
	return nil
}

func (d *Daemon) systemdNotifyStatus(ctx context.Context, status string) {
	message := fmt.Sprintf("STATUS=%s", status)
	//                             ^^
	// You may think that this should be %q, but you'd be wrong!
	// Using %q causes systemctl to print
	//     Status: ""Hello world""
	// Somehow systemd knows to escape spaces so using %s is the right thing to do:
	//     Status: "Hello world"

	sent, err := d.systemdSdNotifier(false, message)
	if err != nil {
		log.Warningf(ctx, "Daemon: couldn't update systemd status to %q: %v", status, err)
		return
	}

	if sent {
		log.Debugf(ctx, "Updated systemd status to %q", status)
	}
}

func clamp(minimum, value, maximum time.Duration) time.Duration {
	return max(minimum, min(value, maximum))
}

// connect connects to the Windows Agent and returns a reverse server.
// Cancel the context to quit gracefully, or Stop the server to abort.
func (d *Daemon) connect(ctx context.Context) (server *streams.Server, err error) {
	defer decorate.OnError(&err, "could not connect to Windows Agent")

	addr, err := d.address(ctx, d.system)
	if err != nil {
		return nil, fmt.Errorf("could not get address: %w", err)
	}

	distroName, err := d.system.WslDistroName(ctx)
	if err != nil {
		log.Warningf(ctx, "Windows host connection: assigning arbitrary connection ID because of error: %v", err)
		distroName = ""
	}

	log.Infof(ctx, "Daemon: starting connection to Windows Agent via %s", addr)

	tlsConfig, err := newTLSConfigFromDir(d.certsPath)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithStreamInterceptor(interceptorschain.StreamClient(
			log.StreamClientInterceptor(logrus.StandardLogger(), log.WithClientID(distroName)),
		)), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("could not create a gRPC client: %v", err)
	}

	return streams.NewServer(ctx, d.system, conn), nil
}

// newTLSConfigFromDir loads certificates from the provided certs path and returns a matching tls.Config.
func newTLSConfigFromDir(certsPath string) (conf *tls.Config, err error) {
	decorate.OnError(&err, "could not load TLS config")

	cert, err := tls.LoadX509KeyPair(filepath.Join(certsPath, common.ClientsCertFilePrefix+common.CertificateSuffix), filepath.Join(certsPath, common.ClientsCertFilePrefix+common.KeySuffix))
	if err != nil {
		return nil, err
	}

	ca := x509.NewCertPool()
	caFilePath := filepath.Join(certsPath, common.RootCACertFileName)
	caBytes, err := os.ReadFile(caFilePath)
	if err != nil {
		return nil, err
	}
	if ok := ca.AppendCertsFromPEM(caBytes); !ok {
		return nil, fmt.Errorf("failed to parse %q", caFilePath)
	}

	return &tls.Config{
		ServerName:   common.GRPCServerNameOverride,
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// address fetches the address of the control stream from the Windows filesystem.
func (d *Daemon) address(ctx context.Context, system *system.System) (string, error) {
	// Parse the port from the file written by the windows agent.
	addr, err := os.ReadFile(d.addressPath)
	if err != nil {
		return "", fmt.Errorf("could not read agent port file %q: %v", d.addressPath, err)
	}

	port, err := splitPort(string(addr))
	if err != nil {
		return "", err
	}

	windowsLocalhost, err := system.WindowsHostAddress(ctx)
	if err != nil {
		return "", streams.NewSystemError("%w", err)
	}

	// Join the address and port, and validate it.
	address := net.JoinHostPort(windowsLocalhost.String(), fmt.Sprint(port))

	return address, nil
}

// splitPort splits the port from the address, and validates that the port is a strictly positive integer.
func splitPort(addr string) (p int, err error) {
	defer decorate.OnError(&err, "could not parse port from %q", addr)

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("could not split address: %v", err)
	}

	p, err = strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("could not parse port as an integer: %v", err)
	}

	if p == 0 {
		return 0, errors.New("port cannot be zero")
	}

	if p < 0 {
		return 0, errors.New("port cannot be negative")
	}

	return p, nil
}
