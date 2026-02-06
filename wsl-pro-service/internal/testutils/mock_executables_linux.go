package testutils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// CmdExe mocks `cmd.exe $args...` on Linux.
// It's special because we want this Cmd to output UTF-16, we cannot go through `go test`,
// as it prevents us from printing arbitrary UTF-16 encoded text.
// Thus, on Linux, we pipe the desired output to iconv (unless an encoding error is desired, for
// which case the relevant control argument will be set in the mock).
func (m *SystemMock) CmdExe(ctx context.Context, path string, args ...string) *exec.Cmd {
	if !testing.Testing() {
		panic("mockExec can only be used within a test")
	}

	code, pipe, output := func() (exitCode, string, string) {
		// Assume we'll output UTF-16LE unless otherwise specified, so we forecast piping
		// the desired output into iconv.
		pipe := " | iconv -f UTF-8 -t UTF-16LE "
		if len(args) < 3 {
			// mock not implemented for arguments
			return exitBadUsage, pipe, fmt.Sprintf("%q: Mock not implemented for args: %s", path, strings.Join(args, ""))
		}
		// The /C and /U flags could come in any relative order and are case insensitive.
		flags := strings.ToLower(strings.Join(args[0:2], ""))
		if (flags != "/u/c" && flags != "/c/u") || (args[2] != "echo.%UserProfile%") {
			// mock not implemented for arguments
			return exitBadUsage, pipe, fmt.Sprintf("%q: Mock not implemented for args: %s", path, strings.Join(args, ""))
		}
		// TODO: Implement the configured errors:
		if _, ok := m.controlArgs[CmdExeErr]; ok {
			return exitError, pipe, "Mock error"
		}

		if _, ok := m.controlArgs[CmdExeEncodingErr]; ok {
			// For this case we'll avoid piping to iconv because we want to output
			// another encoding (UTF-8 here, but could be anything else).
			return exitOk, "", "I am UTF-8 🦄 !\r\n"
		}

		if _, ok := m.controlArgs[EmptyUserprofileEnvVar]; ok {
			// cmd.exe would still print a new line.
			return exitOk, pipe, "\r\n"
		}

		return exitOk, pipe, windowsUserProfileDir
	}()

	if code != exitOk {
		// Print to stderr instead of stdout and exit with specified code.
		//nolint:gosec // G204 - false positive because we control the args (constructed in the closure above).
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("printf '%s' %s >&2; exit %d", output, pipe, code))
	}
	//nolint:gosec // G204 - false positive because we control the args (constructed in the closure above).
	return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("printf '%s' %s ", output, pipe))
}
