package testutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// MockWindowsAgent mocks the windows agent server.
type MockWindowsAgent struct {
	Server   *grpc.Server
	Service  *mockWSLInstanceService
	Listener net.Listener

	ClientCredentials credentials.TransportCredentials

	Started chan struct{}
	Stopped chan struct{}
}

// NewMockWindowsAgent creates a new windows-agent mock. It starts a GRPC service that will perform
// the port dance and stay connected. It'll write the port file as well.
// For simplicity's sake, it only suports one WSL distro at a time.
//
// You can stop it manually, otherwise it'll stop during cleanup.
//
//nolint:revive // testing.T should go before context, regardless of what these linters say.
func NewMockWindowsAgent(t *testing.T, ctx context.Context, publicDir string) *MockWindowsAgent {
	t.Helper()

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp4", "localhost:0")
	require.NoError(t, err, "Setup: could not listen to agent address")

	clientCreds, serverCreds := agentTLSCreds(t, filepath.Join(publicDir, common.CertificatesDir))

	m := MockWindowsAgent{
		Listener:          lis,
		Server:            grpc.NewServer(grpc.Creds(serverCreds)),
		Service:           &mockWSLInstanceService{},
		ClientCredentials: clientCreds,
		Started:           make(chan struct{}),
		Stopped:           make(chan struct{}),
	}
	agentapi.RegisterWSLInstanceServer(m.Server, m.Service)
	t.Cleanup(m.Stop)

	addrFile := filepath.Join(publicDir, common.ListeningPortFileName)
	err = os.WriteFile(addrFile, []byte(lis.Addr().String()), 0600)
	if err != nil {
		close(m.Started)
		close(m.Stopped)
		require.Fail(t, "Setup: could not write listening port file: %v", err)
	}

	go func() {
		log.Infof(ctx, "MockWindowsAgent: Windows-agent mock serving on %s", lis.Addr().String())

		close(m.Started)
		defer close(m.Stopped)

		if err := m.Server.Serve(lis); err != nil {
			log.Infof(ctx, "MockWindowsAgent: Serve returned an error: %v", err)
		}

		if err := os.RemoveAll(addrFile); err != nil {
			log.Infof(ctx, "MockWindowsAgent: Remove address file returned an error: %v", err)
		}
	}()

	<-m.Started

	return &m
}

// agentTLSCreds is a helper that creates a pair of TLS credentials for the agent and the WSL Pro service for testing.
func agentTLSCreds(t *testing.T, destDir string) (wslProService, agentCreds credentials.TransportCredentials) {
	t.Helper()

	require.NoError(t, os.MkdirAll(destDir, 0700), "failed to create certificates directory")

	rootCert, rootKey, err := certs.CreateRootCA("UP4W Test", destDir)
	require.NoError(t, err, "failed to create root CA")

	// Create and write the server and client certificates signed by the root certificate created above.
	agentCert, err := certs.CreateTLSCertificateSignedBy("server", common.GRPCServerNameOverride, rootCert, rootKey, destDir)
	require.NoError(t, err, "failed to create agent certificate", err)
	wslProServiceCert, err := certs.CreateTLSCertificateSignedBy("client", "wsl-pro-service-test", rootCert, rootKey, destDir)
	require.NoError(t, err, "failed to create WSL Pro service certificate", err)

	ca := x509.NewCertPool()
	ca.AddCert(rootCert)
	wslProService = credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS13,
		ServerName:   common.GRPCServerNameOverride,
		Certificates: []tls.Certificate{*wslProServiceCert},
		RootCAs:      ca,
	})
	agentCreds = credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{*agentCert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	})

	return wslProService, agentCreds
}

// Stop releases all resources associated with the MockWindowsAgent.
func (m *MockWindowsAgent) Stop() {
	<-m.Started

	if m.Server != nil {
		m.Server.Stop()
	}

	if m.Listener != nil {
		m.Listener.Close()
	}

	<-m.Stopped
}

type mockWSLInstanceService struct {
	agentapi.UnimplementedWSLInstanceServer

	Connect         channel[agentapi.DistroInfo, int, agentapi.WSLInstance_ConnectedServer]
	ProAttachment   channel[agentapi.MSG, agentapi.ProAttachCmd, agentapi.WSLInstance_ProAttachmentCommandsServer]
	LandscapeConfig channel[agentapi.MSG, agentapi.LandscapeConfigCmd, agentapi.WSLInstance_LandscapeConfigCommandsServer]
}

func (s *mockWSLInstanceService) AllConnected() bool {
	return s.Connect.connected() && s.ProAttachment.connected() && s.LandscapeConfig.connected()
}

func (s *mockWSLInstanceService) AnyConnected() bool {
	return s.Connect.connected() || s.ProAttachment.connected() || s.LandscapeConfig.connected()
}

type receiver[Recv any] interface {
	Recv() (*Recv, error)
}

type sender[Send any] interface {
	Send(*Send) error
}

type channel[Recv any, Send any, Stream grpc.ServerStream] struct {
	callCount   int
	recvHistory []Recv
	stream      *Stream
	mu          sync.Mutex
}

func (ch *channel[Recv, Send, Stream]) connected() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.stream != nil
}

func (ch *channel[Recv, Send, Stream]) History() []*Recv {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	out := make([]*Recv, len(ch.recvHistory))
	for i, rcv := range ch.recvHistory {
		out[i] = &rcv
	}

	return out
}

func (ch *channel[Recv, Send, Stream]) NConnections() int {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.callCount
}

func (ch *channel[Recv, Send, Stream]) Send(msg *Send) error {
	ch.mu.Lock()
	tmp := ch.stream
	ch.mu.Unlock()

	if tmp == nil {
		return errors.New("MockWindowsAgent: not connected")
	}

	snd, ok := any(*tmp).(sender[Send])
	if !ok {
		panic("MockWindowsAgent: this channel cannot send")
	}

	return snd.Send(msg)
}

func (ch *channel[Recv, Send, Stream]) recv() (*Recv, error) {
	ch.mu.Lock()
	tmp := ch.stream
	ch.mu.Unlock()

	if tmp == nil {
		return nil, errors.New("MockWindowsAgent: not connected")
	}

	r, ok := any(*tmp).(receiver[Recv])
	if !ok {
		panic("MockWindowsAgent: this channel cannot receive")
	}

	rcv, err := r.Recv()
	if err != nil {
		return nil, err
	}

	ch.mu.Lock()
	ch.recvHistory = append(ch.recvHistory, *rcv)
	ch.mu.Unlock()

	return rcv, nil
}

func (ch *channel[Recv, Send, Stream]) set(s Stream, helloMsg *Recv) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.stream = &s
	ch.callCount++
	ch.recvHistory = append(ch.recvHistory, *helloMsg)
}

func (ch *channel[Recv, Send, Stream]) reset() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.stream = nil
}

func (s *mockWSLInstanceService) Connected(stream agentapi.WSLInstance_ConnectedServer) (err error) {
	defer decorate.LogOnError(&err)

	msg, err := stream.Recv()
	if err != nil {
		return err
	} else if msg.GetWslName() == "" {
		return errors.New("MockWindowsAgent: WSL name not provided")
	}

	s.Connect.set(stream, msg)
	defer s.Connect.reset()

	log.Info(stream.Context(), "MockWindowsAgent: Connected ready")

	for {
		_, err := s.Connect.recv()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("MockWindowsAgent: Connected stopped: %v", err)
		}
	}
}

func (s *mockWSLInstanceService) ProAttachmentCommands(stream agentapi.WSLInstance_ProAttachmentCommandsServer) (err error) {
	defer decorate.LogOnError(&err)

	msg, err := stream.Recv()
	if err != nil {
		return err
	} else if msg.GetWslName() == "" {
		return errors.New("MockWindowsAgent: WSL name not provided")
	}

	s.ProAttachment.set(stream, msg)
	defer s.ProAttachment.reset()

	log.Info(stream.Context(), "MockWindowsAgent: ProAttachmentCommands ready")

	for {
		_, err := s.ProAttachment.recv()
		if errors.Is(err, io.EOF) {
			log.Info(stream.Context(), "MockWindowsAgent: ProAttachmentCommands finished")
			return nil
		} else if err != nil {
			return fmt.Errorf("MockWindowsAgent: ProAttachmentCommands stopped: %v", err)
		}
	}
}

func (s *mockWSLInstanceService) LandscapeConfigCommands(stream agentapi.WSLInstance_LandscapeConfigCommandsServer) (err error) {
	defer decorate.LogOnError(&err)

	msg, err := stream.Recv()
	if err != nil {
		return err
	} else if msg.GetWslName() == "" {
		return errors.New("MockWindowsAgent: WSL name not provided")
	}

	s.LandscapeConfig.set(stream, msg)
	defer s.LandscapeConfig.reset()

	log.Info(stream.Context(), "MockWindowsAgent: LandscapeConfigCommands ready")

	for {
		_, err := s.LandscapeConfig.recv()
		if errors.Is(err, io.EOF) {
			log.Info(stream.Context(), "MockWindowsAgent: LandscapeConfigCommands finished")
			return nil
		} else if err != nil {
			return fmt.Errorf("MockWindowsAgent: LandscapeConfigCommands stopped: %v", err)
		}
	}
}
