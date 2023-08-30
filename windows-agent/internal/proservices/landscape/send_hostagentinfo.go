package landscape

import (
	"context"
	"errors"
	"fmt"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
)

// SendUpdatedInfo sends a message to the Landscape server with updated
// info about the machine and the distros.
func (c *Client) SendUpdatedInfo(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not send updated info to landscape")

	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.conn == nil {
		return errors.New("disconnected")
	}

	if err := c.sendUpdatedInfo(c.conn); err != nil {
		return err
	}

	return nil
}

func (c *Client) sendUpdatedInfo(conn *connection) error {
	info, err := c.newHostAgentInfo(conn.ctx)
	if err != nil {
		return fmt.Errorf("could not assemble message: %v", err)
	}

	if err := conn.grpcClient.Send(info); err != nil {
		return fmt.Errorf("could not send message: %v", err)
	}

	return nil
}

// newHostAgentInfo assembles a HostAgentInfo message.
func (c *Client) newHostAgentInfo(ctx context.Context) (info *landscapeapi.HostAgentInfo, err error) {
	token, _, err := c.conf.Subscription(ctx)
	if err != nil {
		return info, err
	}

	distros := c.db.GetAll()
	var instances []*landscapeapi.HostAgentInfo_InstanceInfo
	for _, d := range distros {
		instanceInfo, err := newInstanceInfo(d)

		if errors.As(err, &newInstanceInfoMinorError{}) {
			log.Warningf(ctx, "Skipping from landscape info: %v", err)
			continue
		}

		if err != nil {
			log.Errorf(ctx, "Skipping from landscape info: %v", err)
			continue
		}

		instances = append(instances, instanceInfo)
	}

	info = &landscapeapi.HostAgentInfo{
		Token:     token,
		Uid:       c.getUID(),
		Hostname:  c.hostname,
		Instances: instances,
	}

	return info, nil
}

type newInstanceInfoMinorError struct {
	err error
}

func (e newInstanceInfoMinorError) Error() string {
	return e.err.Error()
}

// newInstanceInfo initializes a Instances_InstanceInfo from a distro.
func newInstanceInfo(d *distro.Distro) (info *landscapeapi.HostAgentInfo_InstanceInfo, err error) {
	state, err := d.State()
	if err != nil {
		return info, err
	}

	var instanceState landscapeapi.InstanceState
	switch state {
	case gowsl.Running:
		instanceState = landscapeapi.InstanceState_Running
	case gowsl.Stopped:
		instanceState = landscapeapi.InstanceState_Stopped
	case gowsl.Installing, gowsl.NonRegistered, gowsl.Uninstalling:
		return nil, newInstanceInfoMinorError{err: fmt.Errorf("distro %q is in state %q. Only %q and %q are accepted", d.Name(), state, gowsl.Running, gowsl.Stopped)}
	default:
		return nil, fmt.Errorf("distro %q is in unknown state %q", d.Name(), state)
	}

	properties := d.Properties()
	info = &landscapeapi.HostAgentInfo_InstanceInfo{
		Id:            d.Name(),
		Name:          properties.Hostname,
		VersionId:     properties.VersionID,
		InstanceState: instanceState,
	}

	return info, nil
}
