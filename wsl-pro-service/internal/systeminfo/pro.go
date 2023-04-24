package systeminfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/ubuntu/decorate"
)

// ProStatus returns whether this distro is pro-attached.
func (s System) ProStatus(ctx context.Context) (attached bool, services []string, err error) {
	exe, args := s.backend.ProExecutable("status", "--format=json")
	//nolint:gosec // In production code, these variables are hard-coded (except for the token).
	out, err := exec.CommandContext(ctx, exe, args...).Output()
	if err != nil {
		return attached, services, fmt.Errorf("pro status command returned error: %v\nStdout:%s", err, string(out))
	}

	var attachedStatus struct {
		Attached bool
		Services []struct {
			Name   string
			Status string
		}
	}

	if err = json.Unmarshal(out, &attachedStatus); err != nil {
		return attached, services, fmt.Errorf("could not parse output of pro status: %v\nOutput: %s", err, string(out))
	}

	for _, s := range attachedStatus.Services {
		if s.Status != "enabled" {
			continue
		}
		services = append(services, s.Name)
	}

	return attachedStatus.Attached, services, nil
}

// ProAttach attaches the current distro to Ubuntu Pro.
func (s *System) ProAttach(ctx context.Context, token string) error {
	/*
		We don't parse the json from `pro attach` because stdout is polluted:
		$ pro attach token --format json
		Unable to determine current instance-id
		{"_schema_version": "0.1", "errors": [], "failed_services": [], "needs_reboot": false, "processed_services": [], "result": "success", "warnings": []}
	*/

	exe, args := s.backend.ProExecutable("attach", token, "--format=json")
	//nolint:gosec // In production code, these variables are hard-coded (except for the token).
	out, err := exec.CommandContext(ctx, exe, args...).Output()
	if err != nil {
		return fmt.Errorf("command returned error: %v\nOutput:%s", err, string(out))
	}

	return nil
}

// ProDetach detaches the current distro from Ubuntu Pro.
// If the distro was already detached, nothing is done.
func (s *System) ProDetach(ctx context.Context) error {
	exe, args := s.backend.ProExecutable("detach", "--assume-yes", "--format=json")
	//nolint:gosec // In production code, these variables are hard-coded (except for the token).
	out, detachErr := exec.CommandContext(ctx, exe, args...).Output()
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

// ProEnablement enables or disables a Ubuntu Pro service.
func (s *System) ProEnablement(ctx context.Context, service string, enable bool) (err error) {
	verb := "enable"
	if !enable {
		verb = "disable"
	}
	defer decorate.OnError(&err, "could not %v service %q", verb, service)

	cmd, args := s.backend.ProExecutable(verb, service, "--assume-yes", "--format=json")
	//nolint:gosec // In production code, these variables are hard-coded (except for the service).
	out, err := exec.CommandContext(ctx, cmd, args...).Output()
	if err == nil {
		return nil
	}

	// pro enable/disable returns error if the service was already enabled/disabled.
	// We want this function to be indempotent so we must catch these cases and return no error.
	var response struct {
		Errors []struct {
			MessageCode string `json:"message_code"`
			Message     string
		}
	}

	if e := json.Unmarshal(out, &response); e != nil {
		return errors.Join(
			fmt.Errorf("error: %v. Stdout: %s", err, string(out)),
			fmt.Errorf("could not parse json: %v", e),
		)
	}

	if len(response.Errors) == 0 {
		return errors.Join(
			fmt.Errorf("error: %v. Stdout: %s", err, string(out)),
			errors.New("no errors specified in response"),
		)
	}

	target := "service-already-enabled"
	if !enable {
		target = "service-already-disabled"
	}

	err = nil
	for _, e := range response.Errors {
		if e.MessageCode == target {
			return nil
		}
		err = errors.Join(err, fmt.Errorf("%s: %s", e.MessageCode, e.Message))
	}

	return err
}
