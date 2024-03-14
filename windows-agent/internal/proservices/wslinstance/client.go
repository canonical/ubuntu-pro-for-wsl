package wslinstance

import (
	"context"
	"errors"
	"sync"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/ubuntu/decorate"
)

type client struct {
	service *Service
	name    string
	ctx     context.Context
	cancel  context.CancelFunc

	connStream agentapi.WSLInstance_ConnectedServer
	connReady  chan struct{}

	proStream agentapi.WSLInstance_ProAttachmentCommandsServer
	proReady  chan struct{}

	lpeStream agentapi.WSLInstance_LandscapeConfigCommandsServer
	lpeReady  chan struct{}

	mu sync.RWMutex
}

// Client finds or creates a new multi-stream client.
func (s *Service) client(name string) *client {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	client, ok := s.clients[name]
	if ok {
		return client
	}

	return newClient(s, name)
}

// newClient creates a new client.
// Must be called under the streams mutex.
func newClient(s *Service, name string) *client {
	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		name:    name,
		ctx:     ctx,
		cancel:  cancel,
		service: s,

		connReady: make(chan struct{}),
		proReady:  make(chan struct{}),
		lpeReady:  make(chan struct{}),
	}

	s.clients[name] = c
	return c
}

// WaitReady waits for all three streams to be connected.
func (c *client) WaitReady(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not wait for all streams to connect")

	for _, ready := range []chan struct{}{c.connReady, c.proReady, c.lpeReady} {
		select {
		case <-ready:
		case <-c.ctx.Done():
			return errors.New("client closed")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
			return errors.New("timed out")
		}
	}

	return nil
}

// WaitDone waits for the client to be closed.
func (c *client) WaitDone(ctx context.Context) {
	select {
	case <-c.ctx.Done():
	case <-ctx.Done():
	}
}

// Close closes the client and drop it from the global clients map.
func (c *client) Close() {
	c.cancel()

	c.service.clientsMu.Lock()
	defer c.service.clientsMu.Unlock()

	delete(c.service.clients, c.name)
}

// resultToError translates a result received via gRPC into an error.
func resultToError(result *agentapi.Result) error {
	if result == nil {
		return errors.New("no result")
	}

	if result.Error == nil {
		return nil
	}

	return errors.New(result.GetError())
}

// SetConnectedStream sets the Connected stream for the client.
// This step is necessary for WaitReady to return.
func (c *client) SetConnectedStream(stream agentapi.WSLInstance_ConnectedServer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connStream != nil {
		return errors.New("stream already connected")
	}

	c.connStream = stream
	close(c.connReady)
	return nil
}
