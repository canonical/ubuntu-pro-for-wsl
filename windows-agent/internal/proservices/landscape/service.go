// Package landscape implements a client to the Landscape Host Agent API service.
package landscape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc/connectivity"
)

// Service orquestrates the Landscape hostagent connection. It lasts for the entire lifetime of the program.
// It creates the executor and ensures there is always an active connection, creating a new one otherwise.
type Service struct {
	ctx     context.Context
	cancel  context.CancelFunc
	running chan struct{}

	db   *database.DistroDB
	conf Config

	// Cached hostName
	hostName string

	// Connection
	conn   *connection
	connMu sync.RWMutex

	// connRetrier is used in order to ask the keepConnected
	// function to try again now (instead of waiting for the retrial
	// time). Do not use directly. Instead use signalRetryConnection().
	connRetrier *retryConnection
}

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeClientConfig() (string, config.Source, error)

	Subscription() (string, config.Source, error)

	LandscapeAgentUID() (string, error)
	SetLandscapeAgentUID(string) error

	Notify(func())
}

type options struct {
	hostname string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// New creates a new Landscape service object.
func New(ctx context.Context, conf Config, db *database.DistroDB, args ...Option) (s *Service, err error) {
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

	ctx, cancel := context.WithCancel(ctx)

	s = &Service{
		ctx:         ctx,
		cancel:      cancel,
		conf:        conf,
		db:          db,
		hostName:    opts.hostname,
		connRetrier: newRetryConnection(),
	}

	s.watchConfigChanges(ctx)

	return s, nil
}

// Connect starts the connection and starts talking to the server.
// Call Stop to deallocate resources.
func (s *Service) Connect() (err error) {
	defer decorate.OnError(&err, "could not connect to Landscape server")

	if s.connected() {
		return errors.New("already connected")
	}

	return s.keepConnected()
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
				cooldown := time.NewTimer(wait)
				defer cooldown.Stop()

				// Waiting before reconnecting
				if wait > minWait {
					log.Infof(s.ctx, "Landscape will attempt to connect in %s", wait)
				}

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
				case <-cooldown.C:
					// We use the cooldown to see if the connection is long-lived.
					// Short-lived connections will be considered a failure.
					// This avoids spamming the server with short-lived connections.
					cooldown.Stop()
					cooldown.Reset(wait)
				}

				// Retrial petitions are all satisfied.
				s.connRetrier.Reset()

				log.Info(s.ctx, "Landscape: connecting")
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

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
					log.Info(s.ctx, "Landscape: reconnection requested")
					s.disconnect()
				case <-connectionDone:
					if cooldown.Stop() {
						// Connection was dropped so fast we'll consider it a failure.
						return errors.New("connection dropped unexpectedly")
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

	_, src, err := s.conf.Subscription()
	if err != nil {
		return nil, fmt.Errorf("skipping connection: could not obtain Ubuntu Pro token: %v", err)
	}
	if src == config.SourceNone {
		return nil, errors.New("skipping connection: no Ubuntu Pro token provided")
	}

	conn, err := newConnection(ctx, s)
	if err != nil {
		return nil, err
	}

	connectionDone := make(chan struct{})
	go func() {
		defer close(connectionDone)
		conn.grpcConn.WaitForStateChange(ctx, connectivity.Ready)
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

// watchConfigChanges watches for config changes to detect if a reconnection is in order.
func (s *Service) watchConfigChanges(ctx context.Context) {
	s.conf.Notify(func() {
		oldSettings, ok := func() (connectionSettings, bool) {
			s.connMu.RLock()
			defer s.connMu.RUnlock()

			if s.conn != nil {
				return s.conn.settings, true
			}
			return connectionSettings{}, false
		}()

		if !ok {
			// Not connected yet
			return
		}

		landscapeConf, err := newLandscapeHostConf(s.conf)
		if err != nil {
			log.Warningf(ctx, "Landscape: config monitor: %v", err)
			return
		}

		newSett := newConnectionSettings(landscapeConf)
		if newSett == oldSettings {
			return
		}

		log.Info(ctx, "Landscape: config monitor: detected configuration change: starting reconnection.")

		s.reconnect()
	})
}

func (s *Service) disconnect() {
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.disconnect()
	}
	s.connMu.Unlock()
}

// The following methods expose some internals for the other components to use.

// signalRetryConnection signals the Landscape client to attempt to connect to Landscape.
// It will not block if there is an ative connection. until the reconnect petition
// has been received.
func (s *Service) reconnect() {
	s.connRetrier.Request()
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
