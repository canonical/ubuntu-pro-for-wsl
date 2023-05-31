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
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a client to the landscape service, served remotely.
type Client struct {
	db         *database.DistroDB
	conf       Config
	grpcClient landscapeapi.LandscapeHostAgent_ConnectClient

	// Cached hostname
	hostname string

	connected atomic.Bool
	cancel    func()
	once      sync.Once
}

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeURL(context.Context) (string, error)
	ProToken(context.Context) (string, error)
}

type options struct {
	hostname string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// NewClient creates a new Client for the Landscape service.
func NewClient(conf Config, db *database.DistroDB, args ...Option) (*Client, error) {
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

	return &Client{
		conf:     conf,
		db:       db,
		hostname: opts.hostname,
	}, nil
}

// Connect starts the connection and starts talking to the server.
// Call disconnect to deallocate resources.
func (c *Client) Connect(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not connect to Landscape")

	if c.Connected() {
		return errors.New("already connected")
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// Deallocating resources if first handshake fails
	defer func() {
		if err == nil {
			return
		}
		c.Disconnect()
	}()

	address, err := c.conf.LandscapeURL(ctx)
	if err != nil {
		return err
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	cl := landscapeapi.NewLandscapeHostAgentClient(conn)
	client, err := cl.Connect(ctx)
	if err != nil {
		return err
	}
	c.grpcClient = client

	// Get ready to receive commands
	c.connected.Store(true)
	go c.receiveCommands(ctx)

	// Send first message
	if err := c.SendUpdatedInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Disconnect terminates the connection and deallocates resources.
func (c *Client) Disconnect() {
	c.once.Do(func() {
		c.cancel()
		c.connected.Store(false)
	})
}

// Connected returns true if the Landscape client managed to connect to the server.
func (c *Client) Connected() bool {
	return c.connected.Load()
}
