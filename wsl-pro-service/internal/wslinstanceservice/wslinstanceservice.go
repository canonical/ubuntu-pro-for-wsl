// Package wslinstanceservice is the implementation of the wsl instance API.
package wslinstanceservice

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"google.golang.org/grpc"
)

// ControlStreamClient is the client to the stream between the Windows Agent and the WSL instance service.
type ControlStreamClient interface {
	Send(*agentapi.DistroInfo) error
}

// Service is the object in charge of commuicating to the Windows agent.
type Service struct {
	ctrlStream ControlStreamClient

	wslserviceapi.UnimplementedWSLServer
}

// RegisterGRPCService returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (s *Service) RegisterGRPCService(ctx context.Context, ctrlStream agentapi.WSLInstance_ConnectedClient) *grpc.Server {
	log.Debug(ctx, "Registering GRPC WSL instance service")
	s.ctrlStream = ctrlStream

	grpcServer := grpc.NewServer()

	wslserviceapi.RegisterWSLServer(grpcServer, s)

	return grpcServer
}

// ProAttach serves ProAttach messages sent by the agent.
func (s *Service) ProAttach(ctx context.Context, info *wslserviceapi.AttachInfo) (*wslserviceapi.Empty, error) {
	log.Infof(ctx, "Received ProAttach call with token %q", info.Token)

	attached, err := systeminfo.ProStatus(ctx)
	if err != nil {
		// TODO: middleware to print errors from task
		log.Errorf(ctx, "Error in ProAttach: ProStatus: %v", err)
		return nil, err
	}

	if attached {
		if err := detachPro(ctx); err != nil {
			log.Errorf(ctx, "Error in ProAttach: detachPro: %v", err)
			return nil, err
		}
	}

	err = attachPro(ctx, info.Token)
	if err != nil {
		log.Errorf(ctx, "Error in ProAttach: attachPro:: %v", err)
		return nil, err
	}

	// Check the status again
	sysinfo, err := systeminfo.Get()
	if err != nil {
		log.Warning(ctx, "Could not gather system info, skipping send-back to the control stream")
		return nil, nil
	}

	if err := s.ctrlStream.Send(sysinfo); err != nil {
		log.Errorf(ctx, "Error in ProAttach: Send:: %v", err)
		return nil, err
	}

	return &wslserviceapi.Empty{}, nil
}

// attachPro attaches the current distro to Ubuntu Pro.
func attachPro(ctx context.Context, token string) error {
	// We don't parse the json from pro attach as it can include some message on the same std output:
	/*
		$ sudo pro attach token --format json
		Unable to determine current instance-id
		{"_schema_version": "0.1", "errors": [], "failed_services": [], "needs_reboot": false, "processed_services": [], "result": "success", "warnings": []}
	*/
	out, err := exec.CommandContext(ctx, "pro", "attach", token, "--format=json").CombinedOutput()
	if err != nil {
		return fmt.Errorf("command returned error: %v\nOutput:%s", err, string(out))
	}

	return nil
}

// attachPro detaches the current distro from Ubuntu Pro.
// If the distro was already detached, nothing is done.
func detachPro(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "sudo", "pro", "detach", "--assume-yes", "--format=json").CombinedOutput()
	if err != nil {
		// check that the error is not that the machine is already detached
		var detachedError struct {
			Errors []struct {
				MessageCode string
				Message     string
			}
		}
		if err = json.Unmarshal(out, &detachedError); err != nil {
			return err
		}

		if len(detachedError.Errors) == 0 {
			return fmt.Errorf("command returned error: %v.\nOutput: %s", err, string(out))
		}

		if detachedError.Errors[0].MessageCode == "unattached" {
			return nil
		}

		return fmt.Errorf("command returned error: %s: %s", detachedError.Errors[0].MessageCode, detachedError.Errors[0].Message)
	}
	return nil
}
