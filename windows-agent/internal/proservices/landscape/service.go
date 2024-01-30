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
)

// Service orquestrates the Landscape hostagent connection. It lasts for the entire lifetime of the program.
// It creates the executor and ensures there is always an active connection, creating a new one otherwise.
type Service struct {
	db   *database.DistroDB
	conf Config

	// stopped indicates that the Client has been stopped and is no longer usable
	stopped chan struct{}
	once    sync.Once

	// Cached hostName
	hostName string

	// Connection
	conn   *connection
	connMu sync.RWMutex

	// retryConnection is used in order to ask the keepConnected
	// function to try again now (instead of waiting for the retrial
	// time). Do not use directly. Instead use signalRetryConnection().
	retryConnection chan struct{}
}

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeClientConfig(context.Context) (string, config.Source, error)

	Subscription(context.Context) (string, config.Source, error)

	LandscapeAgentUID(context.Context) (string, error)
	SetLandscapeAgentUID(context.Context, string) error
}

type options struct {
	hostname string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// New creates a new Landscape service object.
func New(conf Config, db *database.DistroDB, args ...Option) (*Service, error) {
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

	c := &Service{
		conf:            conf,
		db:              db,
		hostName:        opts.hostname,
		stopped:         make(chan struct{}),
		retryConnection: make(chan struct{}),
	}

	return c, nil
}

// Connect starts the connection and starts talking to the server.
// Call Stop to deallocate resources.
func (s *Service) Connect(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not connect to Landscape")

	if s.connected() {
		return errors.New("already connected")
	}

	// Dummy connection to indicate that a first attempt was attempted
	s.conn = &connection{}

	defer func() {
		go s.keepConnected(ctx)
	}()

	// First connection
	conn, err := s.newConnection(ctx)
	if err != nil {
		return err
	}

	s.connMu.Lock()
	s.conn = conn
	s.connMu.Unlock()

	return nil
}

// keepConnected supervises the connection. If it drops, reconnection is attempted.
func (s *Service) keepConnected(ctx context.Context) {
	const growthFactor = 2
	const minWait = time.Second
	const maxWait = 10 * time.Minute
	wait := time.Second

	// The loop body is inside this function so that defers can be used
	keepLoooping := true
	for keepLoooping {
		keepLoooping = func() (keepLooping bool) {
			// Using a timer rather than a time.After to avoid leaking
			// the timer for up to $maxWait.
			tk := time.NewTimer(wait)
			defer tk.Stop()

			select {
			case <-tk.C:
				wait = min(growthFactor*wait, maxWait)
			case <-s.retryConnection:
				wait = minWait
			case <-s.stopped:
				// Stop was called
				return false
			}

			s.connMu.Lock()
			defer s.connMu.Unlock()

			// We don't want a queue of petitions to form. It does not matter why
			// we are retrying: just by virtue of retrying we are fulfilling all
			// requests at once. Hence, we close and reopen to unblock all senders.
			//
			// This is why senders need to use this channel under the connMu mutex
			// See the signalRetryConnection method.
			close(s.retryConnection)
			s.retryConnection = make(chan struct{})

			if s.conn == nil {
				// Stop was called
				return false
			}

			if s.conn.connected() {
				// Connection still active
				return true
			}

			s.conn.disconnect()

			conn, err := s.newConnection(ctx)
			if err != nil {
				log.Warningf(ctx, "Landscape reconnect: %v", err)
				return true
			}

			s.conn = conn
			wait = minWait
			return true
		}()
	}
}

// Stop terminates the connection and deallocates resources.
func (s *Service) Stop(ctx context.Context) {
	s.once.Do(func() {
		close(s.stopped)

		s.connMu.Lock()
		defer s.connMu.Unlock()

		if s.conn != nil {
			s.conn.disconnect()
			s.conn = nil
		}
	})
}

// Controller creates a controler for this service.
func (s *Service) Controller() Controller {
	return Controller{
		serviceConn: s,
		serviceData: s,
	}
}

// newConnection validates we meet necessary client-side requirements before
// starting a new connection to Landscape.
//
// Doing this avoids overloading the server with connections that will be
// immediately rejected.
func (s *Service) newConnection(ctx context.Context) (*connection, error) {
	_, src, err := s.conf.Subscription(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not obtain Ubuntu Pro token: %v", err)
	}
	if src == config.SourceNone {
		return nil, errors.New("no Ubuntu Pro token provided")
	}

	return newConnection(ctx, s)
}

// The following methods expose some internals for the other components to use.

// signalRetryConnection signals the Landscape client to attempt to connect to Landscape.
// It will not block if there is an ative connection. until the reconnect petition
// has been received.
func (s *Service) signalRetryConnection() <-chan struct{} {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	return s.retryConnection
}

func (s *Service) hasStopped() <-chan struct{} {
	return s.stopped
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
