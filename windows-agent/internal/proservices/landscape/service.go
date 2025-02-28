// Package landscape implements a client to the Landscape Host Agent API service.
package landscape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc/connectivity"
)

// Service orquestrates the Landscape hostagent connection. It lasts for the entire lifetime of the program.
// It creates the executor and ensures there is always an active connection, creating a new one if necessary.
type Service struct {
	ctx     context.Context
	cancel  context.CancelFunc
	running chan struct{}

	disabled atomic.Bool

	db   *database.DistroDB
	conf Config

	// Cached hostName
	hostName string
	// Where to store persistent artifacts, such as an imported WSL VHDX.
	homedir string
	// Where to download temporary artifacts
	downloaddir string

	// Connection
	conn   *connection
	connMu sync.RWMutex

	// connRetrier is used in order to ask the keepConnected
	// function to try again now (instead of waiting for the retrial
	// time). Do not use directly. Instead use reconnect().
	connRetrier *retryConnection

	cloudinit CloudInit
}

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeClientConfig() (string, config.Source, error)

	Subscription() (string, config.Source, error)

	LandscapeAgentUID() (string, error)
	SetLandscapeAgentUID(context.Context, string) error
}

// CloudInit is a cloud-init user data writer.
type CloudInit interface {
	WriteDistroData(distroName string, cloudInit string, instanceID string) error
	RemoveDistroData(distroName string) error
}

type options struct {
	hostname    string
	homedir     string
	downloaddir string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// New creates a new Landscape service object.
func New(ctx context.Context, conf Config, db *database.DistroDB, cloudInit CloudInit, args ...Option) (s *Service, err error) {
	defer decorate.OnError(&err, "could not initizalize Landscape service")
	var opts options

	for _, f := range args {
		f(&opts)
	}

	if opts.hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("could not get host name: %v", err)
		}
		opts.hostname = hostname
	}

	if opts.homedir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not locate the user home dir: %v", err)
		}

		opts.homedir = homeDir
	}

	if opts.downloaddir == "" {
		opts.downloaddir = os.TempDir()
	}

	ctx, cancel := context.WithCancel(ctx)

	s = &Service{
		ctx:         ctx,
		cancel:      cancel,
		conf:        conf,
		db:          db,
		hostName:    opts.hostname,
		homedir:     opts.homedir,
		downloaddir: opts.downloaddir,
		connRetrier: newRetryConnection(),
		cloudinit:   cloudInit,
	}

	return s, nil
}

// Connect starts the connection and starts talking to the server.
// Call Stop to deallocate resources.
func (s *Service) Connect() (err error) {
	if s.connected() {
		return errors.New("could not connect to Landscape server: already connected")
	}

	if err := s.keepConnected(); errors.Is(err, noConfigError{}) {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

// keepConnected supervises the connection. It attempts connecting before returning.
// The connection will be re-created if:
// - the active one drops.
// - a reconnection is requested via connRetrier.
func (s *Service) keepConnected() error {
	const growthFactor = 2
	const minWait = time.Second
	const maxWait = 10 * time.Minute
	wait := 0 * time.Second // No wait in the first iteration

	s.running = make(chan struct{})
	started := make(chan error)

	go func() {
		defer close(s.running)

		defer s.disconnect()
		first := true

		for {
			err := func() error {
				var waitCh <-chan time.Time

				if !s.disabled.Load() {
					cooldown := time.NewTimer(wait)
					defer cooldown.Stop()
					waitCh = cooldown.C
					if wait > minWait {
						log.Infof(s.ctx, "Landscape will attempt to connect in %s", wait)
					}
				}

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
				case <-waitCh:
					// We use the cooldown to see if the connection is long-lived.
					// Short-lived connections will be considered a failure.
					// This avoids spamming the server with short-lived connections.
					cooldown := time.NewTimer(wait)
					defer cooldown.Stop()
					waitCh = cooldown.C
				}

				// Retrial petitions are all satisfied.
				s.connRetrier.Reset()

				connectionDone, err := s.connectOnce(s.ctx)

				if first {
					started <- err
					close(started)
					wait = minWait
					first = false
				}

				if err != nil {
					return err
				}

				log.Info(s.ctx, "Landscape: connected")
				s.disabled.Store(false)

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
					log.Info(s.ctx, "Landscape: reconnection requested")
					s.disconnect()
				case <-connectionDone:
					select {
					case <-waitCh:
						// Connection was dropped so fast we'll consider it a failure.
						return errors.New("connection dropped unexpectedly")
					default:
					}
					log.Warningf(s.ctx, "Landscape: connection dropped unexpectedly")
				}

				return nil
			}()

			select {
			case <-s.ctx.Done():
				log.Info(s.ctx, "Landscape: stopped by context")
				return
			default:
			}

			if target := (noConfigError{}); errors.As(err, &target) {
				if s.disabled.Load() {
					// "Landscape: service disabled" already logged.
					continue
				}
				// We only log this once.
				log.Infof(s.ctx, "Landscape: service disabled: %v", target)
				s.disabled.Store(true)
				continue
			}

			if err != nil {
				log.Warningf(s.ctx, "Landscape: %v", err)
				wait = min(growthFactor*wait, maxWait)
				continue
			}

			// Connection was long-lived. We don't need to wait before reconnecting.
			wait = minWait
		}
	}()

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case err := <-started:
		return err
	}
}

func (s *Service) connectOnce(ctx context.Context) (<-chan struct{}, error) {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.conn != nil {
		s.conn.disconnect()
		s.conn = nil
	}

	conn, err := newConnection(ctx, s)
	if err != nil {
		return nil, err
	}

	connectionDone := make(chan struct{})
	go func() {
		defer close(connectionDone)

		status := connectivity.Ready // Don't do GetState() just in case we already failed.
		for {
			conn.grpcConn.WaitForStateChange(ctx, status)
			status = conn.grpcConn.GetState()

			if status == connectivity.Shutdown {
				// Connection was closed.
				break
			}
		}
	}()

	s.conn = conn
	return connectionDone, nil
}

// Stop terminates the connection and deallocates resources.
func (s *Service) Stop(ctx context.Context) {
	log.Infof(ctx, "Landscape: stopping")

	s.cancel()
	s.connRetrier.Stop()

	select {
	case <-s.running:
	case <-ctx.Done():
	}
}

// Controller creates a controler for this service.
func (s *Service) Controller() Controller {
	return Controller{
		serviceConn: s,
		serviceData: s,
	}
}

// NotifyUbuntuProUpdate is called when the Ubuntu Pro token changes. It will trigger a reconnection if needed.
func (s *Service) NotifyUbuntuProUpdate(ctx context.Context, token string) {
	s.reconnectIfNewSettings(ctx)
}

// NotifyConfigUpdate is called when the configuration changes. It will trigger a reconnection if needed.
func (s *Service) NotifyConfigUpdate(ctx context.Context, landscapeConf, agentUID string) {
	// We only enable Landscape if there is a UID. Otherwise we disable it (by sending an empty config).
	if agentUID == "" {
		landscapeConf = ""
	}

	if landscapeConf != "" {
		var err error
		landscapeConf, err = filterClientSection(landscapeConf)
		if err != nil {
			log.Errorf(ctx, "Landscape: could not notify config changes: %v", err)
			return
		}
	}

	distributeConfig(ctx, s.db, landscapeConf)
	s.reconnectIfNewSettings(ctx)
}

func (s *Service) reconnectIfNewSettings(ctx context.Context) {
	oldSettings := func() connectionSettings {
		s.connMu.RLock()
		defer s.connMu.RUnlock()

		if s.conn != nil {
			return s.conn.settings
		}
		return connectionSettings{}
	}()

	hostagentConf, err := newLandscapeHostConf(s.conf)
	if err != nil && !errors.Is(err, noConfigError{}) {
		log.Warningf(ctx, "Landscape: config monitor: %v", err)
		return
	}

	newSett := newConnectionSettings(hostagentConf)
	if newSett == oldSettings {
		return
	}

	s.reconnect()
}

func (s *Service) disconnect() {
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.disconnect()
	}
	s.connMu.Unlock()
}

// The following methods expose some internals for the other components to use.

// reconnect signals the Landscape client to attempt to connect to Landscape.
// It will not block if there is an ative connection. until the reconnect petition
// has been received.
func (s *Service) reconnect() {
	s.connRetrier.Request()
}

func (s *Service) connDone() <-chan struct{} {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	if s.conn == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return s.conn.ctx.Done()
}

func (s *Service) hasStopped() <-chan struct{} {
	return s.ctx.Done()
}

func (s *Service) config() Config {
	return s.conf
}

func (s *Service) database() *database.DistroDB {
	return s.db
}

func (s *Service) hostname() string {
	return s.hostName
}

func (s *Service) homeDir() string {
	return s.homedir
}

func (s *Service) downloadDir() string {
	return s.downloaddir
}

func (s *Service) connected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	return s.conn.connected()
}

func (s *Service) sendInfo(info *landscapeapi.HostAgentInfo) error {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	return s.conn.sendInfo(info)
}

func (s *Service) isDisabled() bool {
	return s.disabled.Load()
}

func (s *Service) cloudInit() CloudInit {
	return s.cloudinit
}
