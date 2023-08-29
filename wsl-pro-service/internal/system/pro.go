package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// ProStatus returns whether this distro is pro-attached.
func (s System) ProStatus(ctx context.Context) (attached bool, err error) {
	exe, args := s.backend.ProExecutable("status", "--format=json")
	//nolint:gosec // In production code, these variables are hard-coded (except for the token).
	out, err := exec.CommandContext(ctx, exe, args...).Output()
	if err != nil {
		return false, fmt.Errorf("pro status command returned error: %v\nStdout:%s", err, string(out))
	}

	var attachedStatus struct {
		Attached bool
	}
	if err = json.Unmarshal(out, &attachedStatus); err != nil {
		return false, fmt.Errorf("could not parse output of pro status: %v\nOutput: %s", err, string(out))
	}

	return attachedStatus.Attached, nil
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
