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

type InstanceInfo struct {
	ID            string
	Name          string
	VersionID     string
	InstanceState landscapeapi.InstanceState
}

type HostInfo struct {
	UID       string
	Hostname  string
	Token     string
	Instances []InstanceInfo
}

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

// Service is a mock server for the landscape API which can:
// - Record all received messages.
// - Send commands to the connected clients.
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
// This mock simply logs all the connections it received.
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

		s.recvLog = append(s.recvLog, hostInfo)

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

// MessageLog allows looking into the history if messages received by the server.
func (s *Service) MessageLog() (log []HostInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]HostInfo{}, s.recvLog...)
}

func (s *Service) Hosts() (hosts map[string]HostInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hosts = make(map[string]HostInfo)
	for uid, host := range s.hosts {
		hosts[uid] = host.info
	}

	return hosts
}

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
