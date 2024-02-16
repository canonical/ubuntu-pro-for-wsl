package controlstream

import (
	"context"
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// session represents a connection to the control stream. Every time the connection drops,
// the session object is rendered unusable and a new session must be created.
type session struct {
	stream agentapi.WSLInstance_ConnectedClient
	conn   *grpc.ClientConn
}

// newSession starts a connection to the control stream. Call close to release resources.
func newSession(ctx context.Context, address string) (s session, err error) {
	log.Infof(ctx, "Connecting to control stream at %q", address)

	s.conn, err = grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptorschain.StreamClient(
			log.StreamClientInterceptor(logrus.StandardLogger()),
		)))

	if err != nil {
		return session{}, fmt.Errorf("could not dial: %v", err)
	}

	client := agentapi.NewWSLInstanceClient(s.conn)
	s.stream, err = client.Connected(ctx)
	if err != nil {
		return session{}, fmt.Errorf("could not connect to GRPC service: %v", err)
	}

	return s, nil
}

// close stops the connection (if there is one) and releases resources.
func (s *session) close() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

// send sends a DistroInfo message.
func (s session) send(sysinfo *agentapi.DistroInfo) error {
	if s.stream == nil {
		return errors.New("could not send system info: disconnected")
	}
	if err := s.stream.Send(sysinfo); err != nil {
		return fmt.Errorf("could not send system info: %v", err)
	}
	return nil
}

// recv blocks until a message from the agent is received.
func (s session) recv() (*agentapi.Port, error) {
	if s.stream == nil {
		return nil, errors.New("could not receive a port: disconnected")
	}
	return s.stream.Recv()
}
