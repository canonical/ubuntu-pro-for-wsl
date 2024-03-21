package system

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
	return os.Getenv("WSL_DISTRO_NAME")
}

// ProExecutable returns the full command to run the pro executable with the provided arguments.
func (b realBackend) ProExecutable(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "pro", args...)
}

func (b realBackend) LandscapeConfigExecutable(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "landscape-config", args...)
}

// ProExecutable returns the full command to run the wslpath executable with the provided arguments.
func (b realBackend) WslpathExecutable(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "wslpath", args...)
}

// WslinfoExecutable returns the full command to run the wslinfo executable with the provided arguments.
func (b realBackend) WslinfoExecutable(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "wslinfo", args...)
}

func (b realBackend) CmdExe(ctx context.Context, path string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...)

	// cmd.exe must run within the Windows filesystem to avoid warnings.
	cmd.Dir = filepath.Dir(path)

	return cmd
}

func (b realBackend) LookupGroup(name string) (*user.Group, error) {
	return user.LookupGroup("landscape")
}
