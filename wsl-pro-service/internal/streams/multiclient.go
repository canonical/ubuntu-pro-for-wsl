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
	mainStream agentapi.WSLInstance_ConnectedClient
	proStream  agentapi.WSLInstance_ProAttachmentCommandsClient
	lpeStream  agentapi.WSLInstance_LandscapeConfigCommandsClient
}

// connect connects to the three streams. Call Close to release resources.
func connect(ctx context.Context, conn *grpc.ClientConn) (c *multiClient, err error) {
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

// ProAttachStream is a getter for the ProAttachmentCmd stream.
func (s *multiClient) ProAttachStream() stream[agentapi.ProAttachCmd] {
	return stream[agentapi.ProAttachCmd]{
		grpcStream: s.proStream,
	}
}

// LandscapeConfigStream is a getter for the LandscapeConfigCmd stream.
func (s *multiClient) LandscapeConfigStream() stream[agentapi.LandscapeConfigCmd] {
	return stream[agentapi.LandscapeConfigCmd]{
		grpcStream: s.lpeStream,
	}
}

type grpcStream[Command any] interface {
	Context() context.Context
	Recv() (*Command, error)
	Send(r *agentapi.MSG) error
}

// stream provides a restricted interface for sending and receiving messages.
type stream[Command any] struct {
	grpcStream[Command]
}

func (s stream[Command]) SendResult(err error) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return s.Send(&agentapi.MSG{
		Data: &agentapi.MSG_Result{
			Result: errMsg,
		},
	})
}

func (s stream[Command]) SendWslName(wslName string) error {
	return s.Send(&agentapi.MSG{
		Data: &agentapi.MSG_WslName{
			WslName: wslName,
		},
	})
}
