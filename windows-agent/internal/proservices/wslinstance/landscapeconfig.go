package wslinstance

import (
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/ubuntu/decorate"
)

// LandscapeConfigCommands serves the homonymous stream.
func (s *Service) LandscapeConfigCommands(stream agentapi.WSLInstance_LandscapeConfigCommandsServer) (err error) {
	defer decorate.OnError(&err, "WslInstance: could not handle landscape config commands")
	ctx := stream.Context()

	client, err := commandHandshake(ctx, s, stream.Recv)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.SetLandscapeConfigStream(stream); err != nil {
		return err
	}

	if err := client.WaitReady(ctx); err != nil {
		return err
	}

	// Block until the connection drops
	client.WaitDone(ctx)
	return nil
}

// SetLandscapeConfigStream sets the landscape config stream for the client.
// This step is necessary for WaitReady to return.
func (c *client) SetLandscapeConfigStream(stream agentapi.WSLInstance_LandscapeConfigCommandsServer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lpeStream != nil {
		return errors.New("stream already connected")
	}

	c.lpeStream = stream
	close(c.lpeReady)
	return nil
}

// SendLandscapeConfig sends a landscape config to the client.
// Do not use before the client is ready.
func (c *client) SendLandscapeConfig(config string, uid string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-c.ctx.Done():
		return errors.New("client closed")
	default:
	}

	if c.lpeStream == nil {
		return errors.New("no landscape config stream")
	}

	err := c.lpeStream.Send(&agentapi.LandscapeConfigCmd{
		Config:       config,
		HostagentUid: uid,
	})
	if err != nil {
		c.Close()
		log.Warningf(c.lpeStream.Context(), "LandscapeConfig stream could not send: %v", err)
		return errors.New("could not send landscape config: disconnected")
	}

	result, err := recvContext(c.ctx, c.lpeStream.Recv)
	if err != nil {
		c.Close()
		log.Warningf(c.lpeStream.Context(), "LandscapeConfig stream could not receive: %v", err)
		return errors.New("could not receive landscape config result: disconnected")
	}

	ok, err := msgToError(result)
	if ok {
		return err
	}
	return fmt.Errorf("did not receive landscape config result: %v", err)
}
