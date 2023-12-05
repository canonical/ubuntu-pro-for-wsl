package landscape

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// connection is a proxy for the Landscape server. Lasts until the connection drops, in which case
// a new connection needs to be constructed. Holds no data, but has the methods to send info to
// Landscape, and redirects the received commands to the executor.
type connection struct {
	ctx    context.Context
	cancel func()

	grpcConn   *grpc.ClientConn
	grpcClient landscapeapi.LandscapeHostAgent_ConnectClient
	once       sync.Once

	receivingCommands sync.WaitGroup
}

func (conn *connection) connected() bool {
	if conn == nil {
		return false
	}

	if conn.grpcConn == nil {
		return false
	}

	switch conn.grpcConn.GetState() {
	case connectivity.Idle:
		return false
	case connectivity.Shutdown:
		return false
	}

	return true
}

func (conn *connection) disconnect() {
	// Default constructed connection
	if conn.cancel == nil {
		return
	}

	conn.once.Do(func() {
		conn.cancel()
		conn.receivingCommands.Wait()
		_ = conn.grpcConn.Close()
	})
}

// receiveCommands blocks while the connection is active. It listens for commands from Landscape
// and fowards them to the executor.
func (conn *connection) receiveCommands(e executor) error {
	for {
		select {
		case <-conn.ctx.Done():
			return nil
		default:
		}

		command, err := conn.grpcClient.Recv()
		if errors.Is(err, io.EOF) {
			return errors.New("stream closed by server")
		}
		if err != nil {
			return err
		}

		if err := e.exec(conn.ctx, command); err != nil {
			log.Errorf(conn.ctx, "could not execute command: %v", err)
		}
	}
}

// sendUpdatedInfo takes a HostagentInfo message and forwards it to the Landscape server.
func (conn *connection) sendUpdatedInfo(info *landscapeapi.HostAgentInfo) (err error) {
	defer decorate.OnError(&err, "could not send updated info to Landscape")

	if !conn.connected() {
		return errors.New("disconnected")
	}

	if err := conn.grpcClient.Send(info); err != nil {
		return fmt.Errorf("could not send message: %v", err)
	}

	return nil
}
