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

	// Client UID and where it is stored
	uid      string
	cacheDir string

	connected atomic.Bool
	cancel    func()
	once      sync.Once
}

const cacheFile = "landscape.conf"

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
		conf:     conf,
		db:       db,
		hostname: opts.hostname,
		cacheDir: cacheDir,
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

	address, err := c.conf.LandscapeURL(ctx)
	if err != nil {
		return err
	}

	// Deallocating resources if first handshake fails
	defer func() {
		if err == nil {
			return
		}
		c.Disconnect(ctx)
	}()

	// A context to control the Landscape client with (needed for as long as the connection lasts)
	clientCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// A context to control only the Dial (only needed for this function)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	cl := landscapeapi.NewLandscapeHostAgentClient(conn)
	client, err := cl.Connect(clientCtx)
	if err != nil {
		return err
	}
	c.grpcClient = client

	// Get ready to receive commands
	c.connected.Store(true)
	go c.receiveCommands(clientCtx)

	// Send first message
	if err := c.SendUpdatedInfo(clientCtx); err != nil {
		return err
	}

	// Not the first contact between client and server: done!
	if c.uid != "" {
		return nil
	}

	// First contact. Wait to receive a client UID.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			c.Disconnect()
			c.uid = "" // Avoid races where the UID arrives just after cancelling the context
			return fmt.Errorf("Landscape server did not respond with a client UID")
		case <-ticker.C:
		}

		if c.uid != "" {
			// Server sent a UID: success.
			break
		}
	}

	return nil
}

// Disconnect terminates the connection and deallocates resources.
func (c *Client) Disconnect(ctx context.Context) {
	c.once.Do(func() {
		c.cancel()
		c.waitDisconnected(ctx)
		c.dump()
	})
}

func (c *Client) waitDisconnected(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if connected := c.connected.Load(); !connected {
				return
			}
		}
	}
}

// Connected returns true if the Landscape client managed to connect to the server.
func (c *Client) Connected() bool {
	return c.connected.Load()
}

// load reads persistent Landscape data from disk.
func (c *Client) load() error {
	out, err := os.ReadFile(filepath.Join(c.cacheDir, cacheFile))
	if errors.Is(err, fs.ErrNotExist) {
		// No file: New client
		c.uid = ""
		return nil
	}

	if err != nil {
		// Something is wrong with the file
		return fmt.Errorf("could not read landscape config file: %v", err)
	}

	// First contact done in previous session
	c.uid = string(out)
	return nil
}

// dump stores persistent Landscape data to disk.
func (c *Client) dump() error {
	tmpFile := filepath.Join(c.cacheDir, fmt.Sprintf("%s.tmp", cacheFile))
	cacheFile := filepath.Join(c.cacheDir, cacheFile)

	if err := os.WriteFile(tmpFile, []byte(c.uid), 0600); err != nil {
		return fmt.Errorf("could not store Landscape data to temporary file: %v", err)
	}

	if err := os.Rename(tmpFile, cacheFile); err != nil {
		return fmt.Errorf("could not move Landscape data from tmp to file: %v", err)
	}

	return nil
}
