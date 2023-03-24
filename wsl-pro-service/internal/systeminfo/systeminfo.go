// Package systeminfo contains utils to get system information relevant to
// the Agent.
package systeminfo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"gopkg.in/ini.v1"
)

// System is an object with an easily replaceable back-end that allows accessing
// the filesystem, a few key executables, and some information about the system.
//
// Do not replace the backend after construction, and use one of the provided
// constructors.
type System struct {
	backend Backend // Not embedding to avoid calling its backend directly
}

// Backend is the engine behind the System object, and defines the interactions
// it can perform with the operating system.
type Backend interface {
	Path(p ...string) string
	GetenvWslDistroName() string
	ProExecutable(args ...string) (string, []string)
	WslpathExecutable(args ...string) (string, []string)
}

type options struct {
	backend Backend
}

// Option is an optional argument for New.
type Option = func(*options)

// WithTestBackend is an optional argument for New that injects a backend into the system.
// For testing purposes only.
func WithTestBackend(b Backend) Option {
	return func(o *options) {
		o.backend = b
	}
}

// New instantiates a stateless obejct that mediates interactions with the filesystem
// as well as a few key executables.
func New(args ...Option) System {
	opts := options{backend: realBackend{}}
	for _, f := range args {
		f(&opts)
	}

	//nolint:gosimple // Technically, we could do "return System(opts)", but conceptually they are different objects so I'd rather not.
	return System{
		backend: opts.backend,
	}
}

// Info returns the current information about the system relevant to the GRPC
// connection to the agent.
func (s System) Info(ctx context.Context) (*agentapi.DistroInfo, error) {
	distroName, err := s.wslDistroName(ctx)
	if err != nil {
		return nil, err
	}

	pro, err := s.ProStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not obtain pro status: %v", err)
	}

	info := &agentapi.DistroInfo{
		WslName:     distroName,
		ProAttached: pro,
	}

	if err := s.fillOsRelease(info); err != nil {
		return nil, err
	}

	return info, nil
}

// fillOSRelease fills the info with os-release file content.
func (s System) fillOsRelease(info *agentapi.DistroInfo) error {
	out, err := os.ReadFile(s.backend.Path("etc/os-release"))
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
func (s System) wslDistroName(ctx context.Context) (string, error) {
	// TODO: request Microsoft to expose this to systemd services.
	env := s.backend.GetenvWslDistroName()
	if env != "" {
		return env, nil
	}

	exe, args := s.backend.WslpathExecutable("-w", "/")
	//nolint:gosec //outside of tests, this function simply prepends "wslpath" to the args.
	out, err := exec.CommandContext(ctx, exe, args...).Output()
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

// LocalAppData provides the path to Windows' local app data directory from WSL,
// usually `/mnt/c/Users/JohnDoe/AppData/Local`.
func (s *System) LocalAppData(ctx context.Context) (wslPath string, err error) {
	exe, args := s.backend.WslpathExecutable("-ua", "$(powershell.exe 'echo ${env:LocalAppData}')")
	//nolint:gosec //outside of tests, this function simply prepends "wslpath" to the args.
	out, err := exec.CommandContext(ctx, exe, args...).Output()
	if err != nil {
		return wslPath, fmt.Errorf("error: %v, stdout: %s", err, string(out))
	}
	rawPath := strings.TrimSpace(string(out))
	return s.Path(rawPath), nil
}

// Path converts an absolute path into one inside the mocked filesystem.
func (s System) Path(path ...string) string {
	return s.backend.Path(path...)
}
