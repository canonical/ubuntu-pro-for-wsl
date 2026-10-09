package system

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

type realBackend struct{}

// Path translates an absolute path into its analogous provided for the back-end.
func (b realBackend) Path(p ...string) string {
	return filepath.Join(p...)
}

func (b realBackend) Hostname() (string, error) {
	return os.Hostname()
}

// GetenvWslDistroName obtains the value of environment variable WSL_DISTRO_NAME.
func (b realBackend) GetenvWslDistroName() string {
	name := os.Getenv("WSL_DISTRO_NAME")
	if strings.TrimSpace(name) != "" {
		return name
	}
	return os.Getenv("WSL2_DISTRO_NAME")
}

// GetenvUserProfileDir obtains the value of environment variable WSL2_USER_PROFILE.
func (b realBackend) GetenvUserProfileDir() string {
	return os.Getenv("WSL2_USER_PROFILE")
}

// ProExecutable returns the full command to run the pro executable with the provided arguments.
func (b realBackend) ProExecutable(ctx context.Context, args ...string) *exec.Cmd {
	//#nosec G204 // We control the input variables, there is no risk of command injection.
	return exec.CommandContext(ctx, "pro", args...)
}

func (b realBackend) LandscapeConfigExecutable(ctx context.Context, args ...string) *exec.Cmd {
	//#nosec G204 // We control the input variables, there is no risk of command injection.
	return exec.CommandContext(ctx, "landscape-config", args...)
}

// WslpathExecutable returns the full command to run the wslpath executable with the provided arguments.
// Callers should ensure that any untrusted input is passed as the last argument, as it will be sanitized to prevent command injection.
func (b realBackend) WslpathExecutable(ctx context.Context, args ...string) *exec.Cmd {
	lastIdx := len(args) - 1
	sanitizedArgs := append(args[:lastIdx], "--", strings.TrimSpace(args[lastIdx]))
	//#nosec G204,G702 // We control the input variables, there is litle risk of command injection
	//provided the caller puts any untrusted input as the last argument, thanks to the
	//sanitization done above, enforcing the last argument to be treated as a path string, no
	//matter which shape it has.
	return exec.CommandContext(ctx, "/usr/bin/wslpath", sanitizedArgs...)
}

// WslinfoExecutable returns the full command to run the wslinfo executable with the provided arguments.
func (b realBackend) WslinfoExecutable(ctx context.Context, args ...string) *exec.Cmd {
	//#nosec G204 // We control the input variables, there is no risk of command injection.
	return exec.CommandContext(ctx, "wslinfo", args...)
}

func (b realBackend) CmdExe(ctx context.Context, path string, args ...string) *exec.Cmd {
	//#nosec G204 // We control the input variables, there is no risk of command injection.
	cmd := exec.CommandContext(ctx, path, args...)

	// cmd.exe must run within the Windows filesystem to avoid warnings.
	cmd.Dir = filepath.Dir(path)

	return cmd
}

func (b realBackend) LookupGroup(name string) (*user.Group, error) {
	return user.LookupGroup("landscape")
}
