// Package wslinstance implements the GRPC WSLInstance service.
package wslinstance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/ubuntu/decorate"
)

// LandscapeController is the  controller for the Landscape client proservice.
type LandscapeController interface {
	SendUpdatedInfo(context.Context) error
}

// Service is the WSL Instance GRPC service implementation.
type Service struct {
	agentapi.UnimplementedWSLInstanceServer

	db        *database.DistroDB
	landscape LandscapeController

	clients   map[string]*client
	clientsMu sync.Mutex
}

// New returns a new service handling WSL Instance API.
func New(ctx context.Context, db *database.DistroDB, landscape LandscapeController) (s *Service) {
	log.Debug(ctx, "Building new GRPC WSLInstance server")
	return &Service{
		db:        db,
		landscape: landscape,
		clients:   make(map[string]*client),
	}
}

// Connected establishes a connection with a WSL instance and keeps its properties
// in the database up-to-date.
func (s *Service) Connected(stream agentapi.WSLInstance_ConnectedServer) (err error) {
	ctx := stream.Context()

	client, info, err := mainHandshake(ctx, s, stream.Recv)
	if err != nil {
		return err
	}

	if err := client.SetConnectedStream(stream); err != nil {
		return err
	}
	defer client.Close()

	props, err := propsFromInfo(info)
	if err != nil {
		return fmt.Errorf("invalid DistroInfo: %v", err)
	}

	d, err := s.db.GetDistroAndUpdateProperties(ctx, client.name, props)
	if err != nil {
		return err
	}

	// Update landscape host agent when connecting and disconnecting.
	s.landscapeHostagentSendUpdatedInfo(ctx)
	defer s.landscapeHostagentSendUpdatedInfo(ctx)

	// Wait for other streams to connect
	if err := client.WaitReady(ctx); err != nil {
		return err
	}

	// Load deferred tasks
	d.EnqueueDeferredTasks()

	if err := d.SetConnection(client); err != nil {
		return err
	}

	//nolint:errcheck // We don't care about this error because we're cleaning up
	defer d.SetConnection(nil)

	log.Debug(ctx, "connection to Linux-side WSL service established")

	// Blocking connection for the lifetime of the WSL service.
	for {
		info, err := recvContext(client.ctx, stream.Recv)
		if err != nil {
			return fmt.Errorf("could not receive info: %v", err)
		}

		props, err = propsFromInfo(info)
		if err != nil {
			return fmt.Errorf("invalid DistroInfo: %v", err)
		}

		if d.SetProperties(props) {
			if err := s.db.Dump(); err != nil {
				log.Warningf(ctx, "updating properties: %v", err)
			}
		}

		s.landscapeHostagentSendUpdatedInfo(ctx)
	}
}

func propsFromInfo(info *agentapi.DistroInfo) (props distro.Properties, err error) {
	defer decorate.OnError(&err, "received invalid distribution info")

	if info.GetWslName() == "" {
		return props, errors.New("no WSL name provided in DistroInfo message")
	}

	return distro.Properties{
		DistroID:           info.GetId(),
		VersionID:          info.GetVersionId(),
		PrettyName:         info.GetPrettyName(),
		ProAttached:        info.GetProAttached(),
		Hostname:           info.GetHostname(),
		CreatedByLandscape: info.GetCreatedByLandscape(),
	}, nil
}

// mainHandshake receives the first message from the main stream and attaches the stream to the client.
func mainHandshake(ctx context.Context, s *Service, recv func() (*agentapi.DistroInfo, error)) (c *client, m *agentapi.DistroInfo, err error) {
	recvCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	msg, err := recvContext(recvCtx, recv)
	if err != nil {
		return nil, m, fmt.Errorf("could not start handshake: did not receive: %v", err)
	}

	if msg.GetWslName() == "" {
		return nil, msg, errors.New("could not complete handshake: no WSL name provided")
	}

	return s.client(ctx, msg.GetWslName()), msg, err
}

// commandHandshake receives the first message from a command-sending stream and attaches the stream to the client.
func commandHandshake(ctx context.Context, s *Service, recv func() (*agentapi.MSG, error)) (c *client, err error) {
	recvCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	msg, err := recvContext(recvCtx, recv)
	if err != nil {
		return nil, fmt.Errorf("could not start handshake: did not receive: %v", err)
	}

	name := msg.GetWslName()
	if name == "" {
		return nil, errors.New("could not complete handshake: no WSL name received")
	}

	return s.client(ctx, name), err
}

// recvContext returns as soon as either:
// - A message is received.
// - The context is cancelled.
func recvContext[MessageT any](ctx context.Context, recv func() (*MessageT, error)) (msg *MessageT, err error) {
	type tuple struct {
		msg *MessageT
		err error
	}

	ch := make(chan tuple)
	go func() {
		m, err := recv()
		ch <- tuple{m, err}
		close(ch)
	}()

	select {
	case <-ctx.Done():
		return msg, ctx.Err()
	case m := <-ch:
		return m.msg, m.err
	}
}

// landscapeHostagentSendUpdatedInfo is syntactic sugar to update landscape and
// log in the case error.
func (s *Service) landscapeHostagentSendUpdatedInfo(ctx context.Context) {
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := s.landscape.SendUpdatedInfo(ctx); err != nil {
			log.Warning(ctx, err.Error())
		}
	}()
}
