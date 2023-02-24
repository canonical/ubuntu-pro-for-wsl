// Package systeminfo contains utils to get system information relevant to
// the Agent.
package systeminfo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/canonical/ubuntu-pro-for-windows/agentapi"
	"gopkg.in/ini.v1"
)

// Get returns the current information about the system relevant to the GRPC
// connection to the agent.
func Get() (*agentapi.DistroInfo, error) {
	distroName, err := wslDistroName()
	if err != nil {
		return nil, err
	}

	pro, err := ProStatus(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("could not obtain pro status: %v", err)
	}

	info := &agentapi.DistroInfo{
		WslName:     distroName,
		ProAttached: pro,
	}

	if err := fillOsRelease(info); err != nil {
		return nil, err
	}

	return info, nil
}

// fillOSRelease extends info with os-release file content.
func fillOsRelease(info *agentapi.DistroInfo) error {
	out, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("could not read /etc/os-release file: %v", err)
	}

	var marshaller struct {
		//nolint: revive
		// ini mapper is strict with naming, so we cannot rename Id -> ID as the linter suggests
		Id, VersionId, PrettyName string
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return fmt.Errorf("could not parse /etc/os-release file: %v", err)
	}

	info.PrettyName = marshaller.PrettyName
	info.Id = marshaller.Id
	info.VersionId = marshaller.VersionId

	return nil
}

// TODO: document
func wslDistroName() (string, error) {
	// TODO: request Microsoft to expose this to systemd services.
	env := os.Getenv("WSL_DISTRO_NAME")
	if env != "" {
		return env, nil
	}

	out, err := exec.Command("wslpath", "-w", "/").Output()
	if err != nil {
		return "", fmt.Errorf("could not get distro root path: %v. Stdout: %s", err, string(out))
	}

	// Example output: "\\wsl.localhost\Ubuntu-Preview\"
	fields := strings.Split(string(out), `\`)
	if len(fields) < 4 {
		return "", fmt.Errorf("could not parse distro name from path: %s", string(out))
	}

	return fields[3], nil
}

// ProStatus returns whether Ubuntu Pro is enabled on this distro.
func ProStatus(ctx context.Context) (attached bool, err error) {
	out, err := exec.CommandContext(ctx, "pro", "status", "--format=json").Output()
	if err != nil {
		return false, fmt.Errorf("command returned error: %v\nStdout:%s", err, string(out))
	}

	var attachedStatus struct {
		Attached bool
	}
	if err = json.Unmarshal(out, &attachedStatus); err != nil {
		return false, fmt.Errorf("could not parse output of pro status: %v\nOutput: %s", err, string(out))
	}

	return attachedStatus.Attached, nil
}
