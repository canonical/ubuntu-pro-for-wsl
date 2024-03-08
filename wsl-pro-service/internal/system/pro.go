package system

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ubuntu/decorate"
)

// ProStatus returns whether this distro is pro-attached.
func (s System) ProStatus(ctx context.Context) (attached bool, err error) {
	defer decorate.OnError(&err, "pro status")

	cmd := s.backend.ProExecutable(ctx, "status", "--format=json")
	out, err := runCommand(cmd)
	if err != nil {
		return false, err
	}

	var attachedStatus struct {
		Attached bool
	}
	if err = json.Unmarshal(out, &attachedStatus); err != nil {
		return false, fmt.Errorf("could not parse output: %v. Output: %s", err, string(out))
	}

	return attachedStatus.Attached, nil
}

// ProAttach attaches the current distro to Ubuntu Pro.
func (s *System) ProAttach(ctx context.Context, token string) (err error) {
	defer decorate.OnError(&err, "pro attach")

	/*
		We don't parse the json from `pro attach` because stdout is polluted:
		$ pro attach token --format json
		Unable to determine current instance-id
		{"_schema_version": "0.1", "errors": [], "failed_services": [], "needs_reboot": false, "processed_services": [], "result": "success", "warnings": []}
	*/

	cmd := s.backend.ProExecutable(ctx, "attach", token, "--format=json")
	if _, err := runCommand(cmd); err != nil {
		return err
	}

	return nil
}

// ProDetach detaches the current distro from Ubuntu Pro.
// If the distro was already detached, nothing is done.
func (s *System) ProDetach(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "pro detach")

	cmd := s.backend.ProExecutable(ctx, "detach", "--assume-yes", "--format=json")
	out, detachErr := runCommand(cmd)
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
			return detachErr
		}

		if detachedError.Errors[0].MessageCode == "unattached" {
			return nil
		}

		return fmt.Errorf("command returned error: %s: %s", detachedError.Errors[0].MessageCode, detachedError.Errors[0].Message)
	}
	return nil
}
