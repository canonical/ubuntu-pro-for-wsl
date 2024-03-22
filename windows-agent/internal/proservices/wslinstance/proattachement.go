package wslinstance

import (
	"errors"
	"fmt"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/ubuntu/decorate"
)

// ProAttachmentCommands serves the homonymous stream.
func (s *Service) ProAttachmentCommands(stream agentapi.WSLInstance_ProAttachmentCommandsServer) (err error) {
	defer decorate.OnError(&err, "WslInstance: could not handle pro attachment commands")
	ctx := stream.Context()

	client, err := commandHandshake(ctx, s, stream.Recv)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.SetProAttachmentStream(stream); err != nil {
		return err
	}

	if err := client.WaitReady(ctx); err != nil {
		return err
	}

	// Block until the connection drops
	client.WaitDone(ctx)
	return nil
}

// SetProStream sets the pro attachment stream for the client.
// This step is necessary for WaitReady to return.
func (c *client) SetProAttachmentStream(stream agentapi.WSLInstance_ProAttachmentCommandsServer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.proStream != nil {
		return errors.New("stream already connected")
	}

	c.proStream = stream
	close(c.proReady)
	return nil
}

// SendProAttachment sends a pro attachment token to the client.
// Do not use before the client is ready.
func (c *client) SendProAttachment(proToken string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-c.ctx.Done():
		return errors.New("client closed")
	default:
	}

	if c.proStream == nil {
		return errors.New("no pro attachment stream")
	}

	err := c.proStream.Send(&agentapi.ProAttachCmd{
		Token: proToken,
	})
	if err != nil {
		c.Close()
		log.Warningf(c.proStream.Context(), "ProAttachmentCommands stream could not send: %v", err)
		return errors.New("could not send pro attachment: disconnected")
	}

	msg, err := recvContext(c.ctx, c.proStream.Recv)
	if err != nil {
		c.Close()
		log.Warningf(c.proStream.Context(), "ProAttachmentCommands stream could not receive: %v", err)
		return errors.New("could not receive pro attachment result: disconnected")
	}

	err, ok := msgToError(msg)
	if ok {
		return err
	}
	return fmt.Errorf("did not receive landscape config result: %v", err)
}
