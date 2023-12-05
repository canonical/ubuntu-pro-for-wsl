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
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
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

	if connected(s) {
		return errors.New("already connected")
	}

	// Dummy connection to indicate that a first attempt was attempted
	s.conn = &connection{}

	defer func() {
		go s.keepConnected(ctx)
	}()

	// First connection
	conn, err := newConnection(ctx, s)
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

			conn, err := newConnection(ctx, s)
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

// newConnection attempts to connect to the Landscape server, and blocks until the first
// handshake is complete.
func newConnection(ctx context.Context, d serviceData) (conn *connection, err error) {
	defer decorate.OnError(&err, "could not connect to Landscape")

	conf, err := readLandscapeHostConf(ctx, d.config())
	if err != nil {
		return nil, fmt.Errorf("could not read config: %v", err)
	}
	if conf.hostagentURL == "" {
		return nil, errors.New("no hostagent URL provided in the Landscape configuration")
	}

	conn = &connection{}

	// A context to control the Landscape client with (needed for as long as the connection lasts)
	conn.ctx, conn.cancel = context.WithCancel(ctx)

	// A context to control only the Dial (only needed for this function)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	creds, err := conf.transportCredentials()
	if err != nil {
		return nil, err
	}

	grpcConn, err := grpc.DialContext(dialCtx, conf.hostagentURL, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	conn.grpcConn = grpcConn

	cl := landscapeapi.NewLandscapeHostAgentClient(grpcConn)
	client, err := cl.Connect(conn.ctx)
	if err != nil {
		return nil, err
	}
	conn.grpcClient = client

	// Get ready to receive commands
	conn.receivingCommands.Add(1)
	go func() {
		defer conn.disconnect()
		defer conn.receivingCommands.Done()

		if err := conn.receiveCommands(executor{d}); err != nil {
			log.Warningf(conn.ctx, "Stopped listening for Landscape commands: %v", err)
		} else {
			log.Info(conn.ctx, "Finished listening for Landscape commands.")
		}
	}()

	if err := handshake(ctx, d, conn); err != nil {
		conn.disconnect()
		return nil, err
	}

	log.Infof(ctx, "Connection to Landscape established")

	return conn, nil
}

// handshake executes the first few messages of a connection.
//
// The client introduces itself to the server by sending info to Landscape.
// If this is the first connection ever, the server will respond by assigning
// the host a UID. This Recv is handled by receiveCommands, but handshake
// waits until the UID is received before returning.
func handshake(ctx context.Context, d serviceData, conn *connection) error {
	// Send first message
	info, err := newHostAgentInfo(conn.ctx, d)
	if err != nil {
		return fmt.Errorf("could not assemble message: %v", err)
	}

	if err := conn.sendUpdatedInfo(info); err != nil {
		return err
	}

	conf := d.config()

	// Not the first contact between client and server: done!
	if uid, err := conf.LandscapeAgentUID(ctx); err != nil {
		return err
	} else if uid != "" {
		return nil
	}

	// First contact. Wait to receive a client UID.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(conn.ctx, time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			conn.disconnect()
			// Avoid races where the UID arrives just after cancelling the context
			err := conf.SetLandscapeAgentUID(ctx, "")
			return errors.Join(err, fmt.Errorf("Landscape server did not respond with a client UID"))
		case <-ticker.C:
		}

		if uid, err := conf.LandscapeAgentUID(ctx); err != nil {
			return err
		} else if uid != "" {
			// UID received: success.
			break
		}
	}

	return nil
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

func (s *Service) connection() (conn *connection, release func()) {
	s.connMu.RLock()
	return s.conn, s.connMu.RUnlock
}
