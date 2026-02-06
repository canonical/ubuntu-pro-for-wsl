package testutils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// CmdExe mocks `cmd.exe $args...` on Windows.
// It's special because we want this Cmd to output UTF-16, we cannot go through `go test`,
// as it prevents us from priting arbitrary UTF-16 encoded text.
// Thus, on Windows, we use the real cmd.exe with the /U flag (unless an encoding error is desired,
// for which case the relevant control argument will be set in the mock).
func (m *SystemMock) CmdExe(ctx context.Context, path string, args ...string) *exec.Cmd {
	if !testing.Testing() {
		panic("mockExec can only be used within a test")
	}

	code, flags, output := func() (exitCode, string, string) {
		// Assune we'll output UTF-16LE unless otherwise specified, so we forecast piping
		// the desired output into iconv.
		realFlags := "/U"
		// The /U and /C flags could only come in this specific order but are case insensitive.
		flags := strings.ToLower(strings.Join(args[0:2], ""))
		if (flags != "/u/c") || (args[2] != "echo.%UserProfile%") {
			// mock not implemented for arguments
			return exitBadUsage, realFlags, fmt.Sprintf("%q: Mock not implemented for args: %s", path, strings.Join(args, ""))
		}
		if _, ok := m.controlArgs[CmdExeErr]; ok {
			return exitError, realFlags, "Mock error"
		}

		if _, ok := m.controlArgs[CmdExeEncodingErr]; ok {
			// For this case we'll avoid the /U flag because we want to output
			// another encoding (the system's active ANSI CP here, but could be anything else).
			return exitOk, "", "I am an arbitrarily CP encoded message 🦄 !\r\n"
		}

		if _, ok := m.controlArgs[EmptyUserprofileEnvVar]; ok {
			// cmd.exe would still print a new line.
			return exitOk, realFlags, "\r\n"
		}

		return exitOk, realFlags, windowsUserProfileDir
	}()

	if code != exitOk {
		// Print to stderr instead of stdout and exit with specified code.
		// The single ampersand in cmd.exe implies executing the second command despite the
		// result of the first.
		//nolint:gosec // G204 - false positive because we control the args (constructed in the closure above).
		return exec.CommandContext(ctx, "C:\\Windows\\System32\\cmd.exe", flags, "/C", fmt.Sprintf("echo.'%s' 1>&2 & exit %d", output, code))
	}
	//nolint:gosec // G204 - false positive because we control the args (constructed in the closure above).
	return exec.CommandContext(ctx, "C:\\Windows\\System32\\cmd.exe", flags, "/C", fmt.Sprintf("echo.'%s'", output))
}
