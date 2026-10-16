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

// WslpathExecutable returns the full command to run the wslpath executable with the provided
// arguments, with the last argument being treated as the path to be converted, preventing option
// injection via the path argument.
func (b realBackend) WslpathExecutable(ctx context.Context, args ...string) *exec.Cmd {
	var sanitizedArgs []string
	argsLen := len(args)
	if argsLen > 0 {
		lastIdx := argsLen - 1
		sanitizedArgs = make([]string, 0, argsLen+1)
		sanitizedArgs = append(sanitizedArgs, args[:lastIdx]...)
		sanitizedArgs = append(sanitizedArgs, "--", strings.TrimSpace(args[lastIdx]))
	}
	//#nosec G204,G702 // We control the input variables, there is little risk of command injection
	// provided the caller puts any untrusted input as the last argument, thanks to the
	// sanitization done above, enforcing the last argument to be treated as a path string, no
	// matter which shape it has.
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
