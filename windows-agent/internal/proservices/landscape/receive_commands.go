package landscape

import (
	"context"
	"fmt"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
)

func (c *Client) receiveCommands(ctx context.Context) {
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

		if err := exec(ctx, command); err != nil {
			log.Errorf(ctx, "could not execute command: %v", err)
		}
	}
}

func exec(ctx context.Context, command *landscapeapi.Command) (err error) {
	defer decorate.OnError(&err, "could not execute command %s", commandString(command))

	switch cmd := command.Cmd.(type) {
	case *landscapeapi.Command_Start_:
		return cmdStart(ctx, cmd.Start)
	case *landscapeapi.Command_Stop_:
		return cmdStop(ctx, cmd.Stop)
	case *landscapeapi.Command_Install_:
		return cmdInstall(ctx, cmd.Install)
	case *landscapeapi.Command_Uninstall_:
		return cmdUninstall(ctx, cmd.Uninstall)
	case *landscapeapi.Command_SetDefault_:
		return cmdSetDefault(ctx, cmd.SetDefault)
	case *landscapeapi.Command_ShutdownHost_:
		return cmdShutdownHost(ctx, cmd.ShutdownHost)
	default:
		return fmt.Errorf("unknown command type %T: %v", command.Cmd, command.Cmd)
	}
}

func commandString(command *landscapeapi.Command) string {
	switch cmd := command.Cmd.(type) {
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

func cmdStart(ctx context.Context, cmd *landscapeapi.Command_Start) error {
	panic("not implemented")
}

func cmdStop(ctx context.Context, cmd *landscapeapi.Command_Stop) error {
	panic("not implemented")
}

func cmdInstall(ctx context.Context, cmd *landscapeapi.Command_Install) error {
	panic("not implemented")
}

func cmdUninstall(ctx context.Context, cmd *landscapeapi.Command_Uninstall) error {
	panic("not implemented")
}

func cmdSetDefault(ctx context.Context, cmd *landscapeapi.Command_SetDefault) error {
	d := gowsl.NewDistro(ctx, cmd.GetId())
	return d.SetAsDefault()
}

//nolint:unparam // cmd is not used, but it is passed as an argument to stick to the pattern
func cmdShutdownHost(ctx context.Context, cmd *landscapeapi.Command_ShutdownHost) error {
	return gowsl.Shutdown(ctx)
}
