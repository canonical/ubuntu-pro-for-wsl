// Package landscape implements a client to the Landscape Host Agent API service.
package landscape

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a client to the landscape service, served remotely.
type Client struct {
	db   *database.DistroDB
	conf Config

	// stoped indicates that the Client has been stopped and is no longer usable
	stopped chan struct{}
	once    sync.Once

	// Cached hostname
	hostname string

	// Client UID and where it is stored
	uid       atomic.Value
	cacheFile string

	// Connection
	conn   *connection
	connMu sync.RWMutex
}

type connection struct {
	ctx    context.Context
	cancel func()

	grpcConn   *grpc.ClientConn
	grpcClient landscapeapi.LandscapeHostAgent_ConnectClient
	once       sync.Once

	receivingCommands sync.WaitGroup
}

const cacheFileBase = "landscape.conf"

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeURL(context.Context) (string, error)
	Subscription(context.Context) (string, config.SubscriptionSource, error)
}

type options struct {
	hostname string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// NewClient creates a new Client for the Landscape service.
func NewClient(conf Config, db *database.DistroDB, cacheDir string, args ...Option) (*Client, error) {
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

	c := &Client{
		conf:      conf,
		db:        db,
		hostname:  opts.hostname,
		cacheFile: filepath.Join(cacheDir, cacheFileBase),
		stopped:   make(chan struct{}),
	}

	if err := c.load(); err != nil {
		return nil, err
	}

	return c, nil
}

// Connect starts the connection and starts talking to the server.
// Call disconnect to deallocate resources.
func (c *Client) Connect(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not connect to Landscape")

	if c.Connected() {
		return errors.New("already connected")
	}

	// Dummy connection to indicate that a first attempt was attempted
	c.conn = &connection{}
	defer func() {
		go c.keepConnected(ctx)
	}()

	address, err := c.conf.LandscapeURL(ctx)
	if err != nil {
		return err
	}

	// First connection
	conn, err := c.connect(ctx, address)
	if err != nil {
		return err
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	return nil
}

// keepConnected supervises the connection. If it drops, reconnection is attempted.
func (c *Client) keepConnected(ctx context.Context) {
	const growthFactor = 2
	const minWait = time.Second
	const maxWait = 30 * time.Minute
	wait := time.Second

	// The loop body is inside this function so that defers can be used
	for func() (keepLooping bool) {
		tk := time.NewTimer(wait)
		defer tk.Stop()

		select {
		case <-tk.C:
		case <-c.stopped:
			// Stop was called
			return false
		}

		c.connMu.Lock()
		defer c.connMu.Unlock()

		if c.conn == nil {
			// Stop was called
			return false
		}

		if c.conn.connected() {
			// Connection still active
			return true
		}

		c.conn.disconnect()

		address, err := c.conf.LandscapeURL(ctx)
		if err != nil {
			log.Warningf(ctx, "Landscape reconnect: could not get Landscape URL: %v", err)
			wait = min(growthFactor*wait, maxWait)
			return true
		}

		conn, err := c.connect(ctx, address)
		if err != nil {
			log.Warningf(ctx, "Landscape reconnect: %v", err)
			wait = min(growthFactor*wait, maxWait)
			return true
		}

		c.conn = conn
		wait = minWait
		return true
	}() {
	}
}

func (c *Client) connect(ctx context.Context, address string) (conn *connection, err error) {
	defer decorate.OnError(&err, "could not connect to address %q", address)

	conn = &connection{}

	// A context to control the Landscape client with (needed for as long as the connection lasts)
	conn.ctx, conn.cancel = context.WithCancel(ctx)

	// A context to control only the Dial (only needed for this function)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	grpcConn, err := grpc.DialContext(dialCtx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		if err := c.receiveCommands(conn); err != nil {
			log.Errorf(conn.ctx, "Landscape receive commands exited: %v", err)
		}
	}()

	if err := c.handshake(conn); err != nil {
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
// the host a UID. This is Recv is handled by receiveCommands, but handshake
// waits until the UID is received before returning.
func (c *Client) handshake(conn *connection) error {
	// Send first message
	if err := c.sendUpdatedInfo(conn); err != nil {
		return err
	}

	// Not the first contact between client and server: done!
	if c.getUID() != "" {
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
			c.setUID("") // Avoid races where the UID arrives just after cancelling the context
			return fmt.Errorf("Landscape server did not respond with a client UID")
		case <-ticker.C:
		}

		if c.getUID() != "" {
			// UID received: success.
			break
		}
	}

	return nil
}

// Stop terminates the connection and deallocates resources.
func (c *Client) Stop(ctx context.Context) {
	c.once.Do(func() {
		close(c.stopped)

		c.connMu.Lock()
		defer c.connMu.Unlock()

		if c.conn != nil {
			c.conn.disconnect()
			c.conn = nil
		}

		if err := c.dump(); err != nil {
			log.Errorf(ctx, "Landscape client: %v", err)
		}
	})
}

func (conn *connection) disconnect() {
	// Default constructed connection
	if conn.cancel == nil {
		return
	}

	conn.once.Do(func() {
		conn.cancel()
		conn.receivingCommands.Wait()
		_ = conn.grpcConn.Close()
	})
}

// Connected returns true if the Landscape client managed to connect to the server.
func (c *Client) Connected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	return c.conn.connected()
}

func (conn *connection) connected() bool {
	if conn == nil {
		return false
	}

	if conn.grpcConn == nil {
		return false
	}

	switch conn.grpcConn.GetState() {
	case connectivity.Idle:
		return false
	case connectivity.Shutdown:
		return false
	}

	return true
}

// load reads persistent Landscape data from disk.
func (c *Client) load() error {
	out, err := os.ReadFile(c.cacheFile)
	if errors.Is(err, fs.ErrNotExist) {
		// No file: New client
		c.setUID("")
		return nil
	}

	if err != nil {
		// Something is wrong with the file
		return fmt.Errorf("could not read landscape config file: %v", err)
	}

	// First contact done in previous session
	c.setUID(string(out))
	return nil
}

// dump stores persistent Landscape data to disk.
func (c *Client) dump() error {
	tmpFile := fmt.Sprintf("%s.tmp", c.cacheFile)

	if err := os.WriteFile(tmpFile, []byte(c.getUID()), 0600); err != nil {
		return fmt.Errorf("could not store Landscape data to temporary file: %v", err)
	}

	if err := os.Rename(tmpFile, c.cacheFile); err != nil {
		return fmt.Errorf("could not move Landscape data from tmp to file: %v", err)
	}

	return nil
}

// getUID is syntax sugar to read the UID.
func (c *Client) getUID() string {
	//nolint:forcetypeassert // We know it is going to be a string
	return c.uid.Load().(string)
}

// setUID is syntax sugar to set the UID.
func (c *Client) setUID(s string) {
	c.uid.Store(s)
}
