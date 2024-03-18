// Package streammulticlient encapsulates details of the connection to the control stream served by the Windows Agent.
//
// It only provides communication primitives, it does not handle the logic of the messages themselves.
package streammulticlient

import (
	"context"
	"fmt"
	"sync"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Connected represents a connected client to the Windows Agent.
// It abstracts away the multiple streams into a single object.
//
// It has methods to send and receive messages from the control stream, but
// it does not handle the logic of the messages themselves.
type Connected struct {
	conn *grpc.ClientConn
	once sync.Once

	mainStream agentapi.WSLInstance_ConnectedClient
	proStream  agentapi.WSLInstance_ProAttachmentCommandsClient
	lpeStream  agentapi.WSLInstance_LandscapeConfigCommandsClient
}

// Connect connects to the three streams. Call Close to release resources.
func Connect(ctx context.Context, conn *grpc.ClientConn) (c *Connected, err error) {
	client := agentapi.NewWSLInstanceClient(conn)

	mainStream, err := client.Connected(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not connect to GRPC service: %v", err)
	}
	defer closeOnError(&err, mainStream)

	proStream, err := client.ProAttachmentCommands(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Pro attachment stream: %v", err)
	}
	defer closeOnError(&err, proStream)

	lpeStream, err := client.LandscapeConfigCommands(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Landscape config stream: %v", err)
	}
	defer closeOnError(&err, lpeStream)

	return &Connected{
		conn:       conn,
		mainStream: mainStream,
		proStream:  proStream,
		lpeStream:  lpeStream,
	}, nil
}

// Close stops the connection (if there is one) and releases resources.
func (s *Connected) Close() {
	s.once.Do(func() {
		s.conn.Close()
	})
}

// Done returns a channel that is closed when the connection drops.
func (s *Connected) Done(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{})

	go func() {
		defer close(ch)
		s.conn.WaitForStateChange(ctx, connectivity.Ready)
	}()

	return ch
}

func closeOnError(err *error, closer interface{ CloseSend() error }) {
	if *err != nil {
		_ = closer.CloseSend()
	}
}

// SendInfo sends the distro info via the connected stream.
func (s *Connected) SendInfo(info *agentapi.DistroInfo) error {
	return s.mainStream.Send(info)
}

// Stream provides a restricted interface for sending and receiving messages.
type Stream[Command any] interface {
	Context() context.Context
	Recv() (*Command, error)
	Send(r *agentapi.Result) error
}

// ProAttachStream is a getter for the ProAttachmentCmd stream.
func (s *Connected) ProAttachStream() Stream[agentapi.ProAttachCmd] {
	return s.proStream
}

// LandscapeConfigStream is a getter for the LandscapeConfigCmd stream.
func (s *Connected) LandscapeConfigStream() Stream[agentapi.LandscapeConfigCmd] {
	return s.lpeStream
}
