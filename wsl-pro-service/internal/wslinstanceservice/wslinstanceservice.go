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
	options
}

type options struct {
	proAttachCmd func(ctx context.Context, token string) ([]byte, error)
	proDetachCmd func(ctx context.Context) ([]byte, error)
	proStatus    func(ctx context.Context) (attached bool, err error)

	rootDir string
}

type Option func(*options)

func New(rootDir string, args ...Option) *Service {
	opts := options{
		proAttachCmd: attachProCmd,
		proDetachCmd: detachProCmd,
		proStatus:    systeminfo.ProStatus,
		rootDir:      rootDir,
	}

	for _, f := range args {
		f(&opts)
	}

	return &Service{
		options: opts,
	}
}

// RegisterGRPCService returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (s *Service) RegisterGRPCService(ctx context.Context, ctrlStream ControlStreamClient) *grpc.Server {
	log.Debug(ctx, "Registering GRPC WSL instance service")
	s.ctrlStream = ctrlStream

	grpcServer := grpc.NewServer()

	wslserviceapi.RegisterWSLServer(grpcServer, s)

	return grpcServer
}

// ProAttach serves ProAttach messages sent by the agent.
func (s *Service) ProAttach(ctx context.Context, info *wslserviceapi.AttachInfo) (*wslserviceapi.Empty, error) {
	log.Infof(ctx, "Received ProAttach call with token %q", info.Token)

	attached, err := s.proStatus(ctx)
	if err != nil {
		// TODO: middleware to print errors from task
		log.Errorf(ctx, "Error in ProAttach: ProStatus: %v", err)
		return nil, err
	}

	if attached {
		if err := s.detachPro(ctx); err != nil {
			log.Errorf(ctx, "Error in ProAttach: detachPro: %v", err)
			return nil, err
		}
	}

	err = s.attachPro(ctx, info.Token)
	if err != nil {
		log.Errorf(ctx, "Error in ProAttach: attachPro:: %v", err)
		return nil, err
	}

	log.Debugf(ctx, "ProAttach call: pro attachment complete, sending back result")

	// Check the status again
	sysinfo, err := systeminfo.Get(s.rootDir)
	if err != nil {
		log.Warning(ctx, "Could not gather system info, skipping send-back to the control stream")
		return nil, nil
	}

	if err := s.ctrlStream.Send(sysinfo); err != nil {
		log.Errorf(ctx, "Error in ProAttach: Send:: %v", err)
		return nil, err
	}

	log.Debugf(ctx, "ProAttach call: finished successfully")
	return &wslserviceapi.Empty{}, nil
}

// attachPro attaches the current distro to Ubuntu Pro.
func (s *Service) attachPro(ctx context.Context, token string) error {
	// We don't parse the json from pro attach as it can include some message on the same std output:
	/*
		$ pro attach token --format json
		Unable to determine current instance-id
		{"_schema_version": "0.1", "errors": [], "failed_services": [], "needs_reboot": false, "processed_services": [], "result": "success", "warnings": []}
	*/
	out, err := s.proAttachCmd(ctx, token)
	if err != nil {
		return fmt.Errorf("command returned error: %v\nOutput:%s", err, string(out))
	}

	return nil
}

// attachPro detaches the current distro from Ubuntu Pro.
// If the distro was already detached, nothing is done.
func (s *Service) detachPro(ctx context.Context) error {
	out, detachErr := s.proDetachCmd(ctx)
	if detachErr != nil {
		// check that the error is not that the machine is already detached
		var detachedError struct {
			Errors []struct {
				MessageCode string `json:"message_code"`
				Message     string
			}
		}
		if err := json.Unmarshal(out, &detachedError); err != nil {
			return err
		}

		if len(detachedError.Errors) == 0 {
			return fmt.Errorf("command returned error: %v.\nOutput: %s", detachErr, string(out))
		}

		if detachedError.Errors[0].MessageCode == "unattached" {
			return nil
		}

		return fmt.Errorf("command returned error: %s: %s", detachedError.Errors[0].MessageCode, detachedError.Errors[0].Message)
	}
	return nil
}

func attachProCmd(ctx context.Context, token string) ([]byte, error) {
	return exec.CommandContext(ctx, "pro", "attach", token, "--format=json").Output()
}

func detachProCmd(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "pro", "detach", "--assume-yes", "--format=json").Output()
}
