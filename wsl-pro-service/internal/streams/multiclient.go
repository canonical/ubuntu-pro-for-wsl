package streams

import (
	"context"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"google.golang.org/grpc"
)

// multiClient represents a connected multiClient to the Windows Agent.
// It abstracts away the multiple streams into a single object.
// It only provides communication primitives, it does not handle the logic of the messages themselves.
type multiClient struct {
	conn *grpc.ClientConn

	mainStream agentapi.WSLInstance_ConnectedClient
	proStream  agentapi.WSLInstance_ProAttachmentCommandsClient
	lpeStream  agentapi.WSLInstance_LandscapeConfigCommandsClient
}

// Connect connects to the three streams. Call Close to release resources.
//
//nolint:revive // This method is only public to tests, where multiClient has an alias available
func Connect(ctx context.Context, conn *grpc.ClientConn) (c *multiClient, err error) {
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

	return &multiClient{
		conn:       conn,
		mainStream: mainStream,
		proStream:  proStream,
		lpeStream:  lpeStream,
	}, nil
}

func closeOnError(err *error, closer interface{ CloseSend() error }) {
	if *err != nil {
		_ = closer.CloseSend()
	}
}

// SendInfo sends the distro info via the connected stream.
func (s *multiClient) SendInfo(info *agentapi.DistroInfo) error {
	return s.mainStream.Send(info)
}

// stream provides a restricted interface for sending and receiving messages.
type stream[Command any] interface {
	Context() context.Context
	Recv() (*Command, error)
	Send(r *agentapi.Result) error
}

// ProAttachStream is a getter for the ProAttachmentCmd stream.
func (s *multiClient) ProAttachStream() stream[agentapi.ProAttachCmd] {
	return s.proStream
}

// LandscapeConfigStream is a getter for the LandscapeConfigCmd stream.
func (s *multiClient) LandscapeConfigStream() stream[agentapi.LandscapeConfigCmd] {
	return s.lpeStream
}
