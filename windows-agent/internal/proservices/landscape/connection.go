package landscape

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// connection is a proxy for the Landscape server. Lasts until the connection drops, in which case
// a new connection needs to be constructed. Holds no data, but has the methods to send info to
// Landscape, and redirects the received commands to the executor.
type connection struct {
	settings connectionSettings

	ctx    context.Context
	cancel func()

	grpcConn   *grpc.ClientConn
	grpcClient landscapeapi.LandscapeHostAgent_ConnectClient
	once       sync.Once

	receivingCommands sync.WaitGroup
}

// connectionSettings contains data that is immutable for a connection.
// A change of these settings requires a reconnect.
type connectionSettings struct {
	url             string
	certificatePath string
}

func newConnectionSettings(c landscapeHostConf) connectionSettings {
	return connectionSettings{
		url:             c.hostagentURL,
		certificatePath: c.sslPublicKey,
	}
}

// newConnection attempts to connect to the Landscape server, and blocks until the first
// handshake is complete.
func newConnection(ctx context.Context, d serviceData) (conn *connection, err error) {
	defer decorate.OnError(&err, "could not connect to Landscape server")

	conf, err := newLandscapeHostConf(d.config())
	if err != nil {
		return nil, err
	}
	if conf.hostagentURL == "" {
		return nil, errors.New("no hostagent URL provided in the Landscape configuration")
	}

	// A context to control the Landscape client with (needed for as long as the connection lasts)
	ctx, cancel := context.WithCancel(ctx)

	conn = &connection{
		settings: newConnectionSettings(conf),
		ctx:      ctx,
		cancel:   cancel,
	}

	creds, err := transportCredentials(conn.settings.certificatePath)
	if err != nil {
		return nil, err
	}

	// A context to control only the Dial (only needed for this function)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	grpcConn, err := grpc.DialContext(dialCtx, conn.settings.url, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	conn.grpcConn = grpcConn

	cl := landscapeapi.NewLandscapeHostAgentClient(grpcConn)
	client, err := cl.Connect(ctx)
	if err != nil {
		return nil, err
	}
	conn.grpcClient = client

	// Get ready to receive commands
	conn.receivingCommands.Add(1)
	go func() {
		defer conn.disconnect()
		defer conn.receivingCommands.Done()

		if err := conn.receiveCommands(executor{d}); err != nil {
			log.Warningf(ctx, "Landscape: stopped listening for commands: %v", err)
		} else {
			log.Info(ctx, "Landscape: finished listening for commands.")
		}
	}()

	if err := handshake(ctx, d, conn); err != nil {
		conn.disconnect()
		return nil, err
	}

	return conn, nil
}

// handshake executes the first few messages of a connection.
//
// The client introduces itself to the server by sending info to Landscape.
// If this is the first connection ever, the server will respond by assigning
// the host a UID. This Recv is handled by receiveCommands, but handshake
// waits until the UID is received before returning.
func handshake(ctx context.Context, d serviceData, conn *connection) (err error) {
	defer decorate.OnError(&err, "could not complete handshake")
	log.Debug(ctx, "Landscape: starting handshake")

	// Send first message
	info, err := newHostAgentInfo(conn.ctx, d)
	if err != nil {
		return err
	}

	if err := conn.sendInfo(info); err != nil {
		return err
	}

	conf := d.config()

	// Not the first contact between client and server: done!
	if uid, err := conf.LandscapeAgentUID(); err != nil {
		return err
	} else if uid != "" {
		log.Info(ctx, "Landscape: handshake completed")
		return nil
	}

	log.Debug(ctx, "Landscape: waiting to be assigned a UID")

	// First contact. Wait to receive a client UID.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(conn.ctx, time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			conn.disconnect()
			// Avoid races where the UID arrives just after cancelling the context
			err := conf.SetLandscapeAgentUID("")
			return fmt.Errorf("Landscape server did not respond with a client UID: %v", err)
		case <-ticker.C:
		}

		if uid, err := conf.LandscapeAgentUID(); err != nil {
			return fmt.Errorf("could not ascertain if the server provided a client UID: %v", err)
		} else if uid != "" {
			// UID received: success.
			log.Debugf(ctx, "Landscape: assigned client UID %s", uid)
			break
		}
	}

	log.Debug(ctx, "Landscape: handshake completed")
	return nil
}

// connected returns true if there is an active connection to the Landscape server.
func (conn *connection) connected() bool {
	if conn == nil {
		return false
	}

	if conn.grpcConn == nil {
		return false
	}

	return conn.grpcConn.GetState() == connectivity.Ready
}

// disconnect stops the connection and releases resources.
// This leaves the connection unusable. Create a new connection
// object if you need to re-connect.
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
			return fmt.Errorf("could not receive commands: %v", err)
		}

		if err := e.exec(conn.ctx, command); err != nil {
			log.Errorf(conn.ctx, "Landscape: %v", err)
		}
	}
}

// sendInfo takes a HostagentInfo message and forwards it to the Landscape server.
func (conn *connection) sendInfo(info *landscapeapi.HostAgentInfo) (err error) {
	defer decorate.OnError(&err, "could not send updated info to Landscape")

	if !conn.connected() {
		return errors.New("disconnected")
	}

	if err := conn.grpcClient.Send(info); err != nil {
		return fmt.Errorf("could not send message: %v", err)
	}

	return nil
}
