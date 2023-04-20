// Package testutils implements helper functions for frequently needed functionality
// in tests.
package testutils

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

// SystemInfoMock is used to override systeminfo's behaviour. Its control parameters are not thread safe.
// You can modify them in test setup, but after that you risk a race.
type SystemInfoMock struct {
	// FsRoot is the path to what will be used as root for the test filesystem
	FsRoot string

	// WslDistroNameEnv is the value that the mocked Getenv(WSL_DISTRO_NAME) or wslpath -w / will display
	WslDistroName string

	// WslDistroNameEnvEnabled sets the mocked WSL_DISTRO_NAME to $WslDistroName when true, and to an empty
	// string when false
	WslDistroNameEnvEnabled bool

	// extraEnv are extra environment variables that will be passed to mocked executables
	extraEnv []string
}

var (
	// defaultWindowsMount is the default path used in tests to set the windows filesystem mount.
	defaultWindowsMount = "/mnt/d/"

	// defaultLocalAppDataDir is the default path used in tests to store Windows agent data.
	defaultLocalAppDataDir = filepath.Join(defaultWindowsMount, "/mnt/c/Users/TestUser/AppData/Local/")

	// defaultLocalAppDataDir is the default path used in tests to store the address of the Windows Agent service.
	defaultAddrFile = filepath.Join(defaultLocalAppDataDir, common.LocalAppDataDir, common.ListeningPortFileName)

	//go:embed filesystem_defaults/os-release
	defaultOsReleaseContents []byte

	//go:embed filesystem_defaults/resolv.conf
	defaultResolvConfContents []byte

	//go:embed filesystem_defaults/proc.mounts
	defaultProcMountsContents []byte
)

// controlArg Mock-controlling constants.
type controlArg string

// Arguments that control how the mocked executable will behave.
// If none are provided, the mock will copy the behaviour of the real thing.
const (
	ProStatusErr      = "UP4W_PRO_STATUS_ERR"
	ProStatusBadJSON  = "UP4W_PRO_STATUS_BAD_JSON"
	ProStatusAttached = "UP4W_PRO_STATUS_ATTACHED"

	ProAttachErr = "UP4W_PRO_ATTACH_ERR"

	ProDetachBadJSON = "UP4W_PRO_DETACH_BAD_JSON"

	ProDetachErrAlreadyDetached = "UP4W_PRO_DETACH_ERR_ALREADY_DETACHED"
	ProDetachErrGeneric         = "UP4W_PRO_DETACH_ERR_GENERIC"
	ProDetachErrNoReason        = "UP4W_PRO_DETACH_ERR_UNKNOWN"

	WslpathErr       = "UP4W_WSLPATH_ERR"
	WslpathBadOutput = "UP4W_WSLPATH_BAD_OUTPUT"

	CmdExeErr = "UP4W_CMDEXE_ERR"
)

const (
	// wslpathDistroName indicates what is the name of the distro to the mock wslpath so that
	// it can generate the \\wsl.localhost\<DISTRONAME>\ path.
	//
	// We cannot rely on WSL_DISTRO_NAME because one of the mock options disables it.
	wslpathDistroName = "UP4W_WSLPATH_DISTRONAME"

	// mockExecutable is an environement variable used so the mock executables now they need to
	// be executed instead of being ignored as faux tests.
	mockExecutable = "UP4W_MOCK_EXECUTABLE"
)

// MockSystemInfo sets up a few mocks:
// - filesystem and mock executables for wslpath, pro.
func MockSystemInfo(t *testing.T) (systeminfo.System, *SystemInfoMock) {
	t.Helper()

	mock := &SystemInfoMock{
		FsRoot:                  mockFilesystemRoot(t),
		WslDistroName:           "TEST_DISTRO",
		WslDistroNameEnvEnabled: true,
	}

	return systeminfo.New(systeminfo.WithTestBackend(mock)), mock
}

// DefaultAddrFile is the location where a mocked system will expect the addr file to be located,
// and its containing directory will be created in New().
func (m *SystemInfoMock) DefaultAddrFile() string {
	return m.Path(defaultAddrFile)
}

// SetControlArg adds control arguments to the mock executables.
func (m *SystemInfoMock) SetControlArg(arg controlArg) {
	m.extraEnv = append(m.extraEnv, fmt.Sprintf("%s=1", arg))
}

// Path prepends FsRoot to a path.
func (m *SystemInfoMock) Path(path ...string) string {
	path = append([]string{m.FsRoot}, path...)
	return filepath.Join(path...)
}

// GetenvWslDistroName mocks os.GetEnv("WSL_DISTRO_NAME").
func (m *SystemInfoMock) GetenvWslDistroName() string {
	if m.WslDistroNameEnvEnabled {
		return m.WslDistroName
	}
	return ""
}

// mockExec generates a command of the form `bash -ec <SCRIPT>` that will call an alternate binary
// to the one we are mocking.
//
// At the core of the script we have
//
//	```
//	SWITCH1=1 SWITCH1=2 go test -run <FAUX_TEST> -- <ARGS...>
//	````
//
// The switches control the behaviour of the mock, and FAUX_TEST is the name of a Test* function
// that mocks the behaviour of the executable. The ARGS are the arguments that would be passed to
// the real binary, in this case being passed to the mocked one.
//
// The faux test is in charge of interpreting the switches and the args.
//
// The script has some more boilerplate to trim out text from the testing module.
// In order to make the mock work, the faux test needs to be defined in the test module,
// see the documentation on ProMock for an example.
func (m *SystemInfoMock) mockExec(fauxTestName string, argv ...string) (string, []string) {
	// Switches
	env := make([]string, len(m.extraEnv))
	copy(env, m.extraEnv)
	env = append(env,
		fmt.Sprintf("%s=1", mockExecutable),                      // Ensures the faux test is not skipped
		fmt.Sprintf("%s=%q", wslpathDistroName, m.WslDistroName), // Informs the faux tests what the mock distro name is
	)
	switches := strings.Join(env, " ")

	// Supplanted executable
	exec := fmt.Sprintf("go test -run ^%s$", fauxTestName)

	// Arguments
	for i := range argv {
		argv[i] = fmt.Sprintf("%q", argv[i])
	}
	args := strings.Join(argv, " ")

	// Heart of the script
	script := fmt.Sprintf("%s %s -- %s", switches, exec, args)

	// Trimming testing framework text
	script = fmt.Sprintf("set -o pipefail && %s | head -n -2", script)

	return "bash", []string{"-ec", script}
}

// ProExecutable mocks `pro $args...`.
func (m *SystemInfoMock) ProExecutable(args ...string) (string, []string) {
	return m.mockExec("TestWithProMock", args...)
}

// WslpathExecutable mocks `wslpath $args...`.
func (m *SystemInfoMock) WslpathExecutable(args ...string) (string, []string) {
	return m.mockExec("TestWithWslPathMock", args...)
}

// CmdExe mocks `cmd.exe $args...`.
func (m *SystemInfoMock) CmdExe(path string, args ...string) (string, []string) {
	return m.mockExec("TestWithCmdExeMock", args...)
}

type exitCode int

const (
	exitOk       exitCode = 0  // Mock returns 0
	exitBadUsage exitCode = 5  // Mock was misused
	exitError    exitCode = 99 // Mock returns error as instructed
)

// ProMock mocks the executable for `pro`.
// Add it to your package_test with:
//
//	func TestWithProMock(t *testing.T) { testutils.ProMock(t) }
//
//nolint:thelper // This is a faux test used to mock the executable `pro`
func ProMock(t *testing.T) {
	if t.Name() != "TestWithProMock" {
		panic("The ProMock faux test must be named TestWithProMock")
	}

	mockMain(t, func(argv []string) exitCode {
		if len(argv) == 0 {
			fmt.Fprintln(os.Stderr, "Pro command expects a verb")
			return exitBadUsage
		}

		switch argv[0] {
		case "status":
			if envExists(ProStatusErr) {
				return exitError
			}

			if envExists(ProStatusBadJSON) {
				fmt.Fprintln(os.Stdout, "invalid\nJSON")
				return exitOk
			}

			fmt.Fprintf(os.Stdout, `{"attached": %t, "anotherfield": "potato"}%s`, envExists(ProStatusAttached), "\n")
			return exitOk

		case "attach":
			if envExists(ProAttachErr) {
				fmt.Fprintln(os.Stdout, `{"message": "This error is produced by a mock instructed to fail on pro attach", "message_code": "mock_error"}`)
				return exitError
			}
			return exitOk

		case "detach":
			if envExists(ProDetachBadJSON) {
				fmt.Fprintln(os.Stdout, "invalid\nJSON")
				if envExists(ProDetachErrAlreadyDetached) || envExists(ProDetachErrNoReason) || envExists(ProDetachErrGeneric) {
					return exitError
				}
				return exitOk
			}

			if envExists(ProDetachErrAlreadyDetached) {
				fmt.Fprintln(os.Stdout, `{"errors": [{"message": "This machine is not attached to an Ubuntu Pro subscription.\nSee https://ubuntu.com/pro", "message_code": "unattached", "service": null, "type": "system"}]}`)
				return exitError
			}

			if envExists(ProDetachErrNoReason) {
				fmt.Fprintln(os.Stdout, `{"errors": []}`)
				return exitError
			}

			if envExists(ProDetachErrGeneric) {
				fmt.Fprintln(os.Stdout, `{"errors": [{"message": "This error is produced by a mock instructed to fail on pro detach", "message_code": "mock_error"}]}`)
				return exitError
			}

			return exitOk
		default:
			fmt.Fprintf(os.Stderr, "Unknown verb %q", argv[0])
			return exitBadUsage
		}
	})
}

// WslPathMock mocks the executable for `wslpath`.
// Add it to your package_test with:
//
//	func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
//
//nolint:thelper // This is a faux test used to mock the executable `wslpath`
func WslPathMock(t *testing.T) {
	if t.Name() != "TestWithWslPathMock" {
		panic("The WslPathMock faux test must be named TestWithWslPathMock")
	}

	mockMain(t, func(argv []string) exitCode {
		if len(argv) != 2 {
			fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
			return exitBadUsage
		}

		switch argv[0] {
		case "-w":
			fallthrough
		case "-wa":
			if envExists(WslpathErr) {
				return exitError
			}

			if argv[1] != "/" {
				fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
				return exitBadUsage
			}

			if !envExists(wslpathDistroName) {
				fmt.Fprintf(os.Stderr, "Missing env %q", wslpathDistroName)
				return exitBadUsage
			}

			if envExists(WslpathBadOutput) {
				fmt.Fprintf(os.Stdout, "Bad output\r\nBad\toutput\r\n")
				return exitOk
			}

			fmt.Fprintf(os.Stdout, `\\wsl.localhost\%s\%s`, os.Getenv(wslpathDistroName), "\n")
			return exitOk

		case "-u":
			fallthrough
		case "-ua":
			if envExists(WslpathErr) {
				return exitError
			}

			if argv[1] != defaultLocalAppDataDir {
				fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
				return exitBadUsage
			}

			if envExists(WslpathBadOutput) {
				fmt.Fprintf(os.Stdout, "Bad output\r\nBad\toutput\r\n")
				return exitOk
			}

			fmt.Fprintf(os.Stdout, "%s\r\n", defaultLocalAppDataDir)
			return exitOk

		default:
			fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
			return exitBadUsage
		}
	})
}

// CmdExeMock mocks the executable for `cmd.exe`.
// Add it to your package_test with:
//
//	func TestWithCmdExeMock(t *testing.T) { testutils.CmdExeMock(t) }
//
//nolint:thelper // This is a faux test used to mock the executable `cmd.exe`
func CmdExeMock(t *testing.T) {
	if t.Name() != "TestWithCmdExeMock" {
		panic("The CmdExeMock faux test must be named TestWithCmdExeMock")
	}

	mockMain(t, func(argv []string) exitCode {
		if len(argv) != 2 {
			fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
			return exitBadUsage
		}

		if argv[0] != "/C" {
			fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
			return exitBadUsage
		}

		if argv[1] != "echo %LocalAppData%" {
			fmt.Fprintf(os.Stderr, "Mock not implemented for args %q\n", argv)
			return exitBadUsage
		}

		if envExists(CmdExeErr) {
			return exitError
		}

		fmt.Fprintln(os.Stdout, defaultLocalAppDataDir)
		return exitOk
	})
}

func envExists(arg controlArg) bool {
	return os.Getenv(string(arg)) != ""
}

// mockMain performs boilerplate to mock the main function:
//
//   - ensures all paths end in os.Exit
//
//   - reparses os.Args as:
//
//     go test -run $testName [-- argv...]
//
//nolint:thelper // This is not a real test
func mockMain(t *testing.T, f func(argv []string) exitCode) {
	if !envExists(mockExecutable) {
		t.Skip("Skipped because it is not a real test, but rather a mocked executable")
	}

	var argv []string
	begin := slices.Index(os.Args, "--")
	if begin != -1 {
		argv = os.Args[begin+1:]
	}

	exit := int(f(argv))
	if exit == 0 {
		// testing library only prints this line when it fails
		// Manually printing it means that we can simply remove the last two lines to get the true output
		fmt.Fprintln(os.Stdout, "exit status 0")
	}
	syscall.Exit(exit)
}

// mockFilesystemRoot sets up a skelleton filesystem with files used by the wsl-pro-service and returns
// its root dir.
func mockFilesystemRoot(t *testing.T) (rootDir string) {
	t.Helper()

	rootDir = t.TempDir()

	// Mock /etc/
	err := os.MkdirAll(filepath.Join(rootDir, "etc"), 0750)
	require.NoError(t, err, "Setup: could not create mock /etc/")

	err = os.WriteFile(filepath.Join(rootDir, "etc/resolv.conf"), defaultResolvConfContents, 0600)
	require.NoError(t, err, "Setup: could not write mock /etc/resolv.conf")

	err = os.WriteFile(filepath.Join(rootDir, "etc/os-release"), defaultOsReleaseContents, 0600)
	require.NoError(t, err, "Setup: could not write mock /etc/os-release")

	// Mock /proc/
	err = os.MkdirAll(filepath.Join(rootDir, "/proc"), 0750)
	require.NoError(t, err, "Setup: could not create mock /proc/")

	err = os.WriteFile(filepath.Join(rootDir, "/proc/mounts"), defaultProcMountsContents, 0600)
	require.NoError(t, err, "Setup: could not write mock /proc/mounts")

	// Mock Windows FS
	portDir := filepath.Join(rootDir, defaultAddrFile)
	err = os.MkdirAll(filepath.Dir(portDir), 0750)
	require.NoErrorf(t, err, "Setup: could not create mock %s", portDir)

	system32 := filepath.Join(rootDir, defaultWindowsMount, "WINDOWS/system32")
	err = os.MkdirAll(system32, 0750)
	require.NoError(t, err, "Setup: could not create mock system32")

	err = os.WriteFile(filepath.Join(system32, "cmd.exe"), []byte{}, 0600)
	require.NoError(t, err, "Setup: could not write mock cmd.exe")

	return rootDir
}
