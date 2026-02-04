// Package system contains utils to get system information relevant to
// the Agent.
package system

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

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
	LookupGroup(string) (*user.Group, error)

	ProExecutable(ctx context.Context, args ...string) *exec.Cmd
	LandscapeConfigExecutable(ctx context.Context, args ...string) *exec.Cmd
	WslpathExecutable(ctx context.Context, args ...string) *exec.Cmd
	WslinfoExecutable(ctx context.Context, args ...string) *exec.Cmd

	CmdExe(ctx context.Context, path string, args ...string) *exec.Cmd
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
func New(args ...Option) *System {
	opts := options{backend: realBackend{}}
	for _, f := range args {
		f(&opts)
	}

	s := &System{
		backend: opts.backend,
	}

	return s
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

	cmd := s.backend.WslpathExecutable(ctx, "-w", "/")
	out, err := runCommand(cmd)
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

	// Using the 'echo.' syntax instead of 'echo ' because if %USERPROFILE% was set to empty string it would cause the output to be 'ECHO is on'.
	// With 'echo.%UserProfile%' it correctly prints empty line in that case.
	// Using the /U flag makes it output UTF-16LE, otherwise it would be ANSI, which is
	// code-page dependent, not necessarily ASCII or UTF-8 compliant and unpredictable.
	cmd := s.backend.CmdExe(ctx, cmdExe, "/U", "/C", "echo.%UserProfile%")
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return wslPath, fmt.Errorf("%s: error: %v", cmd.Path, err)
	}

	// This ugly hack is to support testing with mocked cmd.exe. We mock executables by subprocessing a go test binary.
	// As much as I try to force it to output UTF-16, it ends up eating the last NULL byte of the '\n' sequence, resulting in '\r\n' being
	// represented as 0x0d 0x00 0x0a (missing 0x00). So we detect this very specific case and add the missing NULL byte back.
	rawBytes := stdout.Bytes()
	if bytes.Equal(rawBytes[len(rawBytes)-3:], []byte{0x0d, 0x00, 0x0a}) {
		stdout.WriteByte(0x00)
	}
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	reader := transform.NewReader(&stdout, utf16le.NewDecoder())
	var sb strings.Builder
	if _, err = io.Copy(&sb, reader); err != nil {
		return wslPath, err
	}

	trimmed := strings.TrimSpace(sb.String())
	if len(trimmed) == 0 {
		return wslPath, errors.New("%UserProfile% value is empty")
	}
	// We have the path from Windows' perspective ( C:\Users\... )
	// It must be converted to linux ( /mnt/c/Users/... )

	cmd = s.backend.WslpathExecutable(ctx, "-ua", trimmed)
	winHomeLinux, err := runCommand(cmd)
	if err != nil {
		return wslPath, err
	}

	wslPath = s.Path(string(winHomeLinux))

	// wslpath can return invalid paths, so we make sure that it exists
	if s, err := os.Stat(wslPath); err != nil {
		// Stat errors contain the path and the error description
		return wslPath, err
	} else if !s.IsDir() {
		return wslPath, fmt.Errorf("%q is not a directory", wslPath)
	}

	return wslPath, nil
}

// runCommand is a helper that runs a command and returns stdout.
// The first return value is the always trimmed stdout, even in case of error.
// In case of error, both Stdout and Stderr are included in the error message.
func runCommand(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(cmd.Env, "LC_ALL=C") // Ensure that the output is in English

	err := cmd.Run()
	out := bytes.TrimSpace(stdout.Bytes())
	if err != nil {
		return out, fmt.Errorf("%s: error: %v.\n    Stdout: %s\n    Stderr: %s", cmd.Path, err, out, stderr.String())
	}

	return out, nil
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

// groupToGUID searches the group with the specified name and returns its GID.
func (s *System) groupToGUID(name string) (int, error) {
	group, err := s.backend.LookupGroup(name)
	if err != nil {
		return 0, err
	}

	guid, err := strconv.ParseInt(group.Gid, 10, 32)
	if err != nil {
		return 0, errors.New("could not parse %s as an integer")
	}

	return int(guid), nil
}

// currentUser returns the UID of the current user.
func (s *System) currentUser() (int, error) {
	user, err := user.Current()
	if err != nil {
		return 0, err
	}

	userID, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return 0, errors.New("could not parse %s as an integer")
	}

	return int(userID), nil
}
