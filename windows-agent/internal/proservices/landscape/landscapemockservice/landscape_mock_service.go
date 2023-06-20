// Package landscapemockservice implements a mock Landscape service
// DO NOT USE IN PRODUCTION
package landscapemockservice

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
)

// Service is a mock server for the landscape API which can:
// - Record all received messages.
// - Send commands to the connected clients.
type Service struct {
	landscapeapi.UnimplementedLandscapeHostAgentServer
	mu *sync.RWMutex

	// activeConnections maps from hostname to a function to Send commands to that client
	activeConnections map[string]func(*landscapeapi.Command) error

	// recvLog is a log of all received messages
	recvLog []landscapeapi.HostAgentInfo
}

// New constructs and initializes a mock Landscape service.
func New() *Service {
	return &Service{
		mu:                &sync.RWMutex{},
		activeConnections: make(map[string]func(*landscapeapi.Command) error),
	}
}

// Connect implements the Connect API call.
// This mock simply logs all the connections it received.
func (s *Service) Connect(stream landscapeapi.LandscapeHostAgent_ConnectServer) error {
	firstContact := true
	for {
		hostinfo, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("could not receive: %v", err)
		}

		s.mu.Lock()

		if firstContact {
			firstContact = false
			onDisconnect, err := s.firstContact(hostinfo.Uid, stream)
			if err != nil {
				s.mu.Unlock()
				return err
			}
			defer onDisconnect()
		}

		//nolint:govet
		// Copying the mutexes is fine because the public parameters are passed
		// by copy and this code is for tests only.
		s.recvLog = append(s.recvLog, *hostinfo)

		s.mu.Unlock()
	}
}

func (s *Service) firstContact(uid string, stream landscapeapi.LandscapeHostAgent_ConnectServer) (onDisconect func(), err error) {
	if _, ok := s.activeConnections[uid]; ok {
		return nil, fmt.Errorf("Hostname collision: %q", uid)
	}

	// Register the connection so commands can be sent
	ctx, cancel := context.WithCancel(context.Background())
	sendFunc := func(command *landscapeapi.Command) error {
		select {
		case <-ctx.Done():
			return err
		default:
			return stream.Send(command)
		}
	}

	// Assign a UID if none was provided
	if uid == "" {
		//nolint:gosec // No need to be cryptographically secure
		uid = fmt.Sprintf("ServerAssignedUID%x", rand.Int())

		cmd := &landscapeapi.Command_AssignHost_{
			AssignHost: &landscapeapi.Command_AssignHost{
				Uid: uid,
			},
		}
		if err := sendFunc(&landscapeapi.Command{Cmd: cmd}); err != nil {
			cancel()
			return func() {}, err
		}
	}

	s.activeConnections[uid] = sendFunc

	return func() {
		cancel()
		delete(s.activeConnections, uid)
	}, nil
}

// IsConnected checks if a client with the specified hostname has an active connection.
func (s *Service) IsConnected(uid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.activeConnections[uid]
	return ok
}

// SendCommand instructs the server to send a command to the target machine with matching hostname.
func (s *Service) SendCommand(ctx context.Context, uid string, command *landscapeapi.Command) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	send, ok := s.activeConnections[uid]
	if !ok {
		return fmt.Errorf("UID %q not connected", uid)
	}

	return send(command)
}

// MessageLog allows looking into the history if messages received by the server.
func (s *Service) MessageLog() (log []landscapeapi.HostAgentInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]landscapeapi.HostAgentInfo{}, s.recvLog...)
}
