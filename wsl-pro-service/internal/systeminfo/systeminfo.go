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

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"gopkg.in/ini.v1"
)

// DistroNameEnv is the environment variable read to check what the distro name is.
const DistroNameEnv = "WSL_DISTRO_NAME"

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
	out, err := osRelease()
	if err != nil {
		return fmt.Errorf("could not read /etc/os-release file: %v", err)
	}

	var marshaller struct {
		//nolint:revive
		// ini mapper is strict with naming, so we cannot rename Id -> ID as the linter suggests
		Id, VersionId, PrettyName string
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return fmt.Errorf("could not parse /etc/os-release file contents:\n%v", err)
	}

	info.PrettyName = marshaller.PrettyName
	info.Id = marshaller.Id
	info.VersionId = marshaller.VersionId

	return nil
}

// wslDistroName obtains the name of the current WSL distro from these sources
// 1. From environment variable WSL_DISTRO_NAME, as long as it is not empty
// 2. From the Windows path to the distro's root ("\\wsl.localhost\<DISTRO_NAME>\").
func wslDistroName() (string, error) {
	// TODO: request Microsoft to expose this to systemd services.
	env := os.Getenv(DistroNameEnv)
	if env != "" {
		return env, nil
	}

	out, err := wslRootPath()
	if err != nil {
		return "", fmt.Errorf("could not get distro root path: %v. Stdout: %s", err, string(out))
	}

	// Example output for Windows 11: "\\wsl.localhost\Ubuntu-Preview\"
	// Example output for Windows 10: "\\wsl$\Ubuntu-Preview\"
	fields := strings.Split(string(out), `\`)
	if len(fields) < 4 {
		return "", fmt.Errorf("could not parse distro name from path %q", out)
	}

	return fields[3], nil
}

// ProStatus returns whether Ubuntu Pro is enabled on this distro.
func ProStatus(ctx context.Context) (attached bool, err error) {
	out, err := proStatusCmdOutput(ctx)
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

// wslRootPath returns the Windows path to "/". Extracted as a variable to
// allow for dependency injection.
var wslRootPath = func() ([]byte, error) {
	return exec.Command("wslpath", "-w", "/").Output()
}

// proStatusCmdOutput returns the output of `pro status --format=json`. Extracted as a variable to
// allow for dependency injection.
var proStatusCmdOutput = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "pro", "status", "--format=json").Output()
}

// osRelease returns the contents of /etc/os-release. Extracted as a variable to
// allow for dependency injection.
var osRelease = func() ([]byte, error) {
	return os.ReadFile("/etc/os-release")
}
