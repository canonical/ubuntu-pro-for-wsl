// Package system contains utils to get system information relevant to
// the Agent.
package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
)

// System is an object with an easily pluggable back-end that allows accessing
// the filesystem, a few key executables, and some information about the system.
//
// Do not replace the backend after construction, and use one of the provided
// constructors.
type System struct {
	backend Backend // Not embedding to avoid calling its backend directly
	cmdExe  string  // Linux path to cmd.exe

	wslDistroNameCache string
}

// Backend is the engine behind the System object, and defines the interactions
// it can perform with the operating system.
type Backend interface {
	Path(p ...string) string
	Hostname() (string, error)
	GetenvWslDistroName() string
	ProExecutable(args ...string) (string, []string)
	LandscapeConfigExecutable(args ...string) (string, []string)
	WslpathExecutable(args ...string) (string, []string)
	WslinfoExecutable(args ...string) (string, []string)
	CmdExe(path string, args ...string) (string, []string)
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

// New instantiates a stateless object that mediates interactions with the filesystem
// as well as a few key executables.
func New(args ...Option) System {
	opts := options{backend: realBackend{}}
	for _, f := range args {
		f(&opts)
	}

	return System{
		backend: opts.backend,
	}
}

// Info returns the current information about the system relevant to the GRPC
// connection to the agent.
func (s System) Info(ctx context.Context) (*agentapi.DistroInfo, error) {
	distroName, err := s.WslDistroName(ctx)
	if err != nil {
		return nil, err
	}

	pro, err := s.ProStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not obtain pro status: %v", err)
	}

	hostname, err := s.backend.Hostname()
	if err != nil {
		return nil, fmt.Errorf("could not obtain hostname: %v", err)
	}

	info := &agentapi.DistroInfo{
		WslName:     distroName,
		ProAttached: pro,
		Hostname:    hostname,
	}

	if err := s.fillOsRelease(info); err != nil {
		return nil, err
	}

	return info, nil
}

// fillOSRelease fills the info with os-release file content.
func (s System) fillOsRelease(info *agentapi.DistroInfo) error {
	const fileName = "/etc/os-release"

	out, err := os.ReadFile(s.backend.Path(fileName))
	if err != nil {
		return fmt.Errorf("could not read %s: %v", fileName, err)
	}

	var marshaller struct {
		//nolint:revive
		// ini mapper is strict with naming, so we cannot rename Id -> ID as the linter suggests
		Id, VersionId, PrettyName string
	}

	if err := ini.MapToWithMapper(&marshaller, ini.SnackCase, out); err != nil {
		return fmt.Errorf("could not parse %s: %v", fileName, err)
	}

	info.PrettyName = marshaller.PrettyName
	info.Id = marshaller.Id
	info.VersionId = marshaller.VersionId

	return nil
}

// WslDistroName obtains the name of the current WSL distro from these sources
// 1. From environment variable WSL_DISTRO_NAME, as long as it is not empty
// 2. From the Windows path to the distro's root ("\\wsl.localhost\<DISTRO_NAME>\").
func (s *System) WslDistroName(ctx context.Context) (name string, err error) {
	defer decorate.OnError(&err, "could not obtain WSL distro name")

	if s.wslDistroNameCache != "" {
		// Cache hit
		return s.wslDistroNameCache, nil
	}

	// TODO: request Microsoft to expose this to systemd services.
	env := s.backend.GetenvWslDistroName()
	if env != "" {
		return env, nil
	}

	exe, args := s.backend.WslpathExecutable("-w", "/")
	//nolint:gosec //outside of tests, this function simply prepends "wslpath" to the args.
	out, err := exec.CommandContext(ctx, exe, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get distro root path: %v. Output: %s", err, string(out))
	}

	// Example output for Windows 11: "\\wsl.localhost\Ubuntu-Preview\"
	// Example output for Windows 10: "\\wsl$\Ubuntu-Preview\"
	fields := strings.Split(string(out), `\`)
	if len(fields) < 4 {
		return "", fmt.Errorf("could not parse distro name from path %q", out)
	}

	s.wslDistroNameCache = fields[3]
	return s.wslDistroNameCache, nil
}

// UserProfileDir provides the path to Windows' user profile directory from WSL,
// usually `/mnt/c/Users/JohnDoe/`.
func (s *System) UserProfileDir(ctx context.Context) (wslPath string, err error) {
	defer decorate.OnError(&err, "could not locate Windows' user profile directory")

	// Find folder where windows is mounted on
	cmdExe, err := s.findCmdExe()
	if err != nil {
		return wslPath, err
	}

	exe, args := s.backend.CmdExe(cmdExe, "/C", "echo %UserProfile%")
	//nolint:gosec //this function simply prepends the WSL path to "cmd.exe" to the args.
	out, err := exec.CommandContext(ctx, exe, args...).CombinedOutput()
	if err != nil {
		return wslPath, fmt.Errorf("%s: error: %v. Output: %s", exe, err, string(out))
	}

	// Path from Windows' perspective ( C:\Users\... )
	// It must be converted to linux ( /mnt/c/Users/... )
	winHome := strings.TrimSpace(string(out))

	exe, args = s.backend.WslpathExecutable("-ua", winHome)
	//nolint:gosec //outside of tests, this function simply prepends "wslpath" to the args.
	out, err = exec.CommandContext(ctx, exe, args...).CombinedOutput()
	if err != nil {
		return wslPath, fmt.Errorf("%s: error: %v. Output: %s", exe, err, string(out))
	}

	winHomeLinux := strings.TrimSpace(string(out))
	wslPath = s.Path(winHomeLinux)

	// wslpath can return invalid paths, so we make sure that it exists
	if s, err := os.Stat(wslPath); err != nil {
		// Stat errors contain the path and the error description
		return wslPath, err
	} else if !s.IsDir() {
		return wslPath, fmt.Errorf("%q is not a directory", wslPath)
	}

	return wslPath, nil
}

// Path converts an absolute path into one inside the mocked filesystem.
func (s System) Path(path ...string) string {
	return s.backend.Path(path...)
}

// findCmdExe looks at all the mounts for those that could be Windows drives,
// and checks if ${DRIVE}/WINDOWS/system32/cmd.exe exists. If it does, it returns it.
// Err will be non-nil if the search cannot be conducted or if no such path exists.
//
// The result is cached so the search only happens once.
func (s *System) findCmdExe() (cmdExe string, err error) {
	defer decorate.OnError(&err, "could not locate Windows' cmd.exe")

	// Path can be cached
	if s.cmdExe != "" {
		return s.cmdExe, nil
	}
	defer func() { s.cmdExe = cmdExe }()

	const fileName = "/proc/mounts"
	f, err := os.Open(s.backend.Path(fileName))
	if err != nil {
		return "", fmt.Errorf("could not find where the Windows drive is mounted: %v", err)
	}
	defer f.Close()

	const subPath = "WINDOWS/system32/cmd.exe"

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// Fields: Device | Mount point | FsType | other fields we don't care about
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue // Not enough fields
		}

		// Filesystem type
		if fields[2] != "9p" {
			continue
		}

		path := s.backend.Path(fields[1], subPath)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		return path, nil
	}

	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("could not find where the Windows drive is mounted: could not parse %s: %v", fileName, err)
	}

	return "", fmt.Errorf("none of the mounted drives contains subpath %s", subPath)
}
