package system

import (
	"os"
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
func (b realBackend) ProExecutable(args ...string) (string, []string) {
	return "pro", args
}

func (b realBackend) LandscapeConfigExecutable(args ...string) (string, []string) {
	return "landscape-config", args
}

// ProExecutable returns the full command to run the wslpath executable with the provided arguments.
func (b realBackend) WslpathExecutable(args ...string) (string, []string) {
	return "wslpath", args
}

// WslinfoExecutable returns the full command to run the wslinfo executable with the provided arguments.
func (b realBackend) WslinfoExecutable(args ...string) (string, []string) {
	return "wslinfo", args
}

func (b realBackend) CmdExe(path string, args ...string) (string, []string) {
	return path, args
}
