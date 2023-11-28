// Package landscapemockservice implements a mock Landscape service
// DO NOT USE IN PRODUCTION
package landscapemockservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	UID      string
	Hostname string
	Token    string

	AccountName     string
	RegistrationKey string

	Instances []InstanceInfo
}

// receiveHostInfo receives a landscapeapi.HostAgentInfo and converts it to a HostInfo.
func receiveHostInfo(stream landscapeapi.LandscapeHostAgent_ConnectServer) (HostInfo, error) {
	src, err := stream.Recv()
	if err != nil {
		return HostInfo{}, err
	}

	if src == nil {
		return HostInfo{}, errors.New("nil HostAgentInfo")
	}

	h := HostInfo{
		UID:             src.GetUid(),
		Hostname:        src.GetHostname(),
		Token:           src.GetToken(),
		Instances:       make([]InstanceInfo, 0, len(src.GetInstances())),
		AccountName:     src.GetAccountName(),
		RegistrationKey: src.GetRegistrationKey(),
	}

	for _, inst := range src.GetInstances() {
		h.Instances = append(h.Instances, InstanceInfo{
			ID:            inst.GetId(),
			Name:          inst.GetName(),
			VersionID:     inst.GetVersionId(),
			InstanceState: inst.GetInstanceState(),
		})
	}

	return h, nil
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
func (s *Service) Connect(stream landscapeapi.LandscapeHostAgent_ConnectServer) (err error) {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	recv := asyncRecv(ctx, stream)

	// We keep the hostInfo outside the loop so we can log messages with the hostname.
	var hostInfo HostInfo

	firstContact := true
	for {
		var msg recvMsg
		select {
		case msg = <-recv:
		case <-ctx.Done():
			slog.Info(fmt.Sprintf("Landscape: %s: terminated connection: %v", hostInfo.Hostname, ctx.Err()))
			return nil
		}

		if msg.err != nil {
			slog.Info(fmt.Sprintf("Landscape: %s: terminated connection: %v", hostInfo.Hostname, msg.err))
			return err
		}
		hostInfo = msg.info

		s.mu.Lock()

		s.recvLog = append(s.recvLog, hostInfo)

		if firstContact {
			slog.Info(fmt.Sprintf("Landscape: %s: New connection", hostInfo.Hostname))

			firstContact = false
			uid, onDisconnect, err := s.firstContact(ctx, cancel, hostInfo, stream)
			if err != nil {
				s.mu.Unlock()
				slog.Info(fmt.Sprintf("Landscape: %s: terminated connection: %v", hostInfo.Hostname, err))
				return err
			}
			defer onDisconnect()
			hostInfo.UID = uid
		} else {
			slog.Info(fmt.Sprintf("Landscape: %s: Received update: %+v", hostInfo.Hostname, hostInfo))
		}

		h := s.hosts[hostInfo.UID]
		h.info = hostInfo
		s.hosts[hostInfo.UID] = h

		s.mu.Unlock()
	}
}

// recvMsg is the sanitized return type of a GRPC Recv, used to pass by channel.
type recvMsg struct {
	info HostInfo
	err  error
}

// asyncRecv is an asynchronous GRPC Recv.
// Usually, you cannot select between a context and a GRPC receive. This function allows you to.
// It will keep receiving until the context is cancelled.
func asyncRecv(ctx context.Context, stream landscapeapi.LandscapeHostAgent_ConnectServer) <-chan recvMsg {
	ch := make(chan recvMsg)

	go func() {
		defer close(ch)

		for {
			info, err := receiveHostInfo(stream)

			select {
			case <-ctx.Done():
				return
			case ch <- recvMsg{info, err}:
			}
		}
	}()

	return ch
}

func (s *Service) firstContact(ctx context.Context, cancel func(), hostInfo HostInfo, stream landscapeapi.LandscapeHostAgent_ConnectServer) (uid string, onDisconect func(), err error) {
	if s.isConnected(hostInfo.UID) {
		return "", nil, fmt.Errorf("UID collision: %q", hostInfo.UID)
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
			return "", func() {}, err
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

	return s.isConnected(uid)
}

// isConnected is the unsafe version of IsConnected. It checks if a client with the
// specified hostname has an active connection.
func (s *Service) isConnected(uid string) bool {
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

	slog.Info(fmt.Sprintf("Landscape: %s: sending command %T: %v", conn.info.Hostname, command.GetCmd(), command.GetCmd()))

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

// Disconnect kills the connection with the host assigned to the specified UID.
func (s *Service) Disconnect(uid string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, ok := s.hosts[uid]
	if !ok {
		return fmt.Errorf("UID %q not registered", uid)
	}

	slog.Info(fmt.Sprintf("Landscape: %s: requested disconnection", host.info.Hostname))
	host.stop()
	return nil
}
