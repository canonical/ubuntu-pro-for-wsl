package landscape

import (
	"context"
	"errors"
	"fmt"
	"os/user"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
)

func (c *Client) receiveCommands(ctx context.Context) {
	defer c.connected.Store(false)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		command, err := c.grpcClient.Recv()
		if err != nil {
			log.Error(ctx, err.Error())
			return
		}

		if err := c.exec(ctx, command); err != nil {
			log.Errorf(ctx, "could not execute command: %v", err)
		}
	}
}

func (c *Client) exec(ctx context.Context, command *landscapeapi.Command) (err error) {
	defer decorate.OnError(&err, "could not execute command %s", commandString(command))

	switch cmd := command.Cmd.(type) {
	case *landscapeapi.Command_AssignHost_:
		return c.cmdAssignHost(ctx, cmd.AssignHost)
	case *landscapeapi.Command_Start_:
		return c.cmdStart(ctx, cmd.Start)
	case *landscapeapi.Command_Stop_:
		return c.cmdStop(ctx, cmd.Stop)
	case *landscapeapi.Command_Install_:
		return c.cmdInstall(ctx, cmd.Install)
	case *landscapeapi.Command_Uninstall_:
		return c.cmdUninstall(ctx, cmd.Uninstall)
	case *landscapeapi.Command_SetDefault_:
		return c.cmdSetDefault(ctx, cmd.SetDefault)
	case *landscapeapi.Command_ShutdownHost_:
		return c.cmdShutdownHost(ctx, cmd.ShutdownHost)
	default:
		return fmt.Errorf("unknown command type %T: %v", command.Cmd, command.Cmd)
	}
}

func commandString(command *landscapeapi.Command) string {
	switch cmd := command.Cmd.(type) {
	case *landscapeapi.Command_AssignHost_:
		return fmt.Sprintf("Assign host (uid: %q)", cmd.AssignHost.Uid)
	case *landscapeapi.Command_Start_:
		return fmt.Sprintf("Start (id: %q)", cmd.Start.Id)
	case *landscapeapi.Command_Stop_:
		return fmt.Sprintf("Stop (id: %q)", cmd.Stop.Id)
	case *landscapeapi.Command_Install_:
		return fmt.Sprintf("Install (id: %q)", cmd.Install.Id)
	case *landscapeapi.Command_Uninstall_:
		return fmt.Sprintf("Uninstall (id: %q)", cmd.Uninstall.Id)
	case *landscapeapi.Command_SetDefault_:
		return fmt.Sprintf("SetDefault (id: %q)", cmd.SetDefault.Id)
	case *landscapeapi.Command_ShutdownHost_:
		return "ShutdownHost"
	default:
		return "Unknown"
	}
}

func (c *Client) cmdAssignHost(ctx context.Context, cmd *landscapeapi.Command_AssignHost) error {
	if c.getUID() != "" {
		log.Warning(ctx, "Overriding current landscape client UID")
	}

	c.setUID(cmd.Uid)

	if err := c.dump(); err != nil {
		return err
	}

	return nil
}

//nolint:unparam // ctx is not necessary but is here to be consistent with the other commands.
func (c *Client) cmdStart(ctx context.Context, cmd *landscapeapi.Command_Start) (err error) {
	d, ok := c.db.Get(cmd.Id)
	if !ok {
		return fmt.Errorf("distro %q not in database", cmd.Id)
	}

	return d.LockAwake()
}

//nolint:unparam // ctx is not necessary but is here to be consistent with the other commands.
func (c *Client) cmdStop(ctx context.Context, cmd *landscapeapi.Command_Stop) (err error) {
	d, ok := c.db.Get(cmd.Id)
	if !ok {
		return fmt.Errorf("distro %q not in database", cmd.Id)
	}

	return d.ReleaseAwake()
}

func (*Client) cmdInstall(ctx context.Context, cmd *landscapeapi.Command_Install) (err error) {
	if cmd.Cloudinit != nil && *cmd.Cloudinit != "" {
		return fmt.Errorf("Cloud Init support is not yet available")
	}

	distro := gowsl.NewDistro(ctx, cmd.GetId())
	if registered, err := distro.IsRegistered(); err != nil {
		return err
	} else if registered {
		return errors.New("already installed")
	}

	if err := gowsl.Install(ctx, distro.Name()); err != nil {
		return err
	}

	defer func() {
		if err == nil {
			return
		}
		// Avoid error states by cleaning up on error
		err := distro.Uninstall(ctx)
		if err != nil {
			log.Infof(ctx, "Landscape Install: failed to clean up %q after failed Install: %v", distro.Name(), err)
		}
	}()

	if err := distroinstall.InstallFromExecutable(ctx, distro); err != nil {
		return err
	}

	windowsUser, err := user.Current()
	if err != nil {
		return err
	}

	const userID = 1000

	userName := windowsUser.Username
	if !distroinstall.UsernameIsValid(userName) {
		userName = "ubuntu"
	}

	if err := distroinstall.CreateUser(ctx, distro, userName, windowsUser.Name, userID); err != nil {
		return err
	}

	if err := distro.DefaultUID(userID); err != nil {
		return fmt.Errorf("could not set user as default: %v", err)
	}

	return nil
}

func (c *Client) cmdUninstall(ctx context.Context, cmd *landscapeapi.Command_Uninstall) (err error) {
	d, ok := c.db.Get(cmd.Id)
	if !ok {
		return fmt.Errorf("distro %q not in database", cmd.Id)
	}

	return d.Uninstall(ctx)
}

func (*Client) cmdSetDefault(ctx context.Context, cmd *landscapeapi.Command_SetDefault) error {
	d := gowsl.NewDistro(ctx, cmd.GetId())
	return d.SetAsDefault()
}

//nolint:unparam // // cmd is not necessary but is here to be consistent with the other commands.
func (*Client) cmdShutdownHost(ctx context.Context, cmd *landscapeapi.Command_ShutdownHost) error {
	return gowsl.Shutdown(ctx)
}
