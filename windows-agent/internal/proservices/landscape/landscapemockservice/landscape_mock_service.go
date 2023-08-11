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

// InstanceInfo is the same as landscapeapi.InstanceInfo, but without the mutexes and
// all grpc implementation details (so it can be safely copied).
type InstanceInfo struct {
	ID            string
	Name          string
	VersionID     string
	InstanceState landscapeapi.InstanceState
}

// HostInfo is the same as landscapeapi.HostAgentInfo, but without the mutexes and
// all grpc implementation details (so it can be safely copied).
type HostInfo struct {
	UID       string
	Hostname  string
	Token     string
	Instances []InstanceInfo
}

// newHostInfo recursively copies the info in a landscapeapi.HostAgentInfo to a HostInfo.
func newHostInfo(src *landscapeapi.HostAgentInfo) HostInfo {
	h := HostInfo{
		UID:       src.Uid,
		Hostname:  src.Hostname,
		Token:     src.Token,
		Instances: make([]InstanceInfo, 0, len(src.Instances)),
	}

	for _, inst := range src.Instances {
		h.Instances = append(h.Instances, InstanceInfo{
			ID:            inst.Id,
			Name:          inst.Name,
			VersionID:     inst.VersionId,
			InstanceState: inst.InstanceState,
		})
	}

	return h
}

type host struct {
	send      func(*landscapeapi.Command) error
	info      HostInfo
	connected *bool
	stop      func()
}

// Service is a minimalistic server for the landscape API.
type Service struct {
	landscapeapi.UnimplementedLandscapeHostAgentServer
	mu *sync.RWMutex

	// hosts maps from UID to a host
	hosts map[string]host

	// recvLog is a log of all received messages
	recvLog []HostInfo
}

// New constructs and initializes a mock Landscape service.
func New() *Service {
	return &Service{
		mu:    &sync.RWMutex{},
		hosts: make(map[string]host),
	}
}

// Connect implements the Connect API call.
// Upon first contact ever, a UID is randombly assigned to the host and sent to it.
// In subsequent contacts, this UID will be its unique identifier.
func (s *Service) Connect(stream landscapeapi.LandscapeHostAgent_ConnectServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	firstContact := true
	ch := make(chan HostInfo)
	defer close(ch)

	for {
		go func() {
			recv, err := stream.Recv()
			if err != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			ch <- newHostInfo(recv)
		}()

		var hostInfo HostInfo
		select {
		case hostInfo = <-ch:
		case <-ctx.Done():
			return nil
		}

		s.mu.Lock()

		s.recvLog = append(s.recvLog, hostInfo)

		if firstContact {
			firstContact = false
			uid, onDisconnect, err := s.firstContact(ctx, cancel, hostInfo, stream)
			if err != nil {
				s.mu.Unlock()
				return err
			}
			defer onDisconnect()
			hostInfo.UID = uid
		}

		h := s.hosts[hostInfo.UID]
		h.info = hostInfo
		s.hosts[hostInfo.UID] = h

		s.mu.Unlock()
	}
}

func (s *Service) firstContact(ctx context.Context, cancel func(), hostInfo HostInfo, stream landscapeapi.LandscapeHostAgent_ConnectServer) (uid string, onDisconect func(), err error) {
	if other, ok := s.hosts[hostInfo.UID]; ok && other.connected != nil && *other.connected {
		return uid, nil, fmt.Errorf("UID collision: %q", hostInfo.UID)
	}

	// Register the connection so commands can be sent
	sendFunc := func(command *landscapeapi.Command) error {
		select {
		case <-ctx.Done():
			return err
		default:
			return stream.Send(command)
		}
	}

	// Assign a UID if none was provided
	if hostInfo.UID == "" {
		//nolint:gosec // No need to be cryptographically secure
		hostInfo.UID = fmt.Sprintf("ServerAssignedUID%x", rand.Int())

		cmd := &landscapeapi.Command_AssignHost_{
			AssignHost: &landscapeapi.Command_AssignHost{
				Uid: hostInfo.UID,
			},
		}
		if err := sendFunc(&landscapeapi.Command{Cmd: cmd}); err != nil {
			cancel()
			return uid, func() {}, err
		}
	}

	h := host{
		send:      sendFunc,
		stop:      cancel,
		info:      hostInfo,
		connected: new(bool),
	}

	s.hosts[hostInfo.UID] = h
	*h.connected = true

	return hostInfo.UID, func() {
		cancel()
		*h.connected = false
	}, nil
}

// IsConnected checks if a client with the specified hostname has an active connection.
func (s *Service) IsConnected(uid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, ok := s.hosts[uid]
	return ok && host.connected != nil && *host.connected
}

// SendCommand instructs the server to send a command to the target machine with matching hostname.
func (s *Service) SendCommand(ctx context.Context, uid string, command *landscapeapi.Command) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.hosts[uid]
	if !ok {
		return fmt.Errorf("UID %q not connected", uid)
	}

	return conn.send(command)
}

// MessageLog allows looking into the history of messages received by the server.
func (s *Service) MessageLog() (log []HostInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]HostInfo{}, s.recvLog...)
}

// Hosts returns a map of all hosts that have had a UID assigned in the past, and their most
// recently received data.
func (s *Service) Hosts() (hosts map[string]HostInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hosts = make(map[string]HostInfo)
	for uid, host := range s.hosts {
		hosts[uid] = host.info
	}

	return hosts
}

// Disconnect kills the connection the host wit the specified UID.
func (s *Service) Disconnect(uid string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, ok := s.hosts[uid]
	if !ok {
		return fmt.Errorf("UID %q not registered", uid)
	}

	host.stop()
	return nil
}
