package systeminfo_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common/golden"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBehaviour int

const (
	mockOK        mockBehaviour = iota // A mock that mimicks the happy path
	mockError                          // A mock that returns an error
	mockBadOutput                      // A mock that returns a bad value with no error
)

func TestInfo(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// This causes Get to look at the Windows' path for /
		distroNameEnvDisabled bool

		// The path of "/" according to the Windows host
		distroNameWslPath mockBehaviour

		proStatusCommand mockBehaviour
		osRelease        mockBehaviour

		wantErr bool
	}{
		"Success reading from WSL_DISTRO_NAME": {},
		"Success using wslpath":                {distroNameEnvDisabled: true},

		"Error when WSL_DISTRO_NAME is empty and wslpath fails":            {distroNameEnvDisabled: true, distroNameWslPath: mockError, wantErr: true},
		"Error when WSL_DISTRO_NAME is empty and wslpath returns bad text": {distroNameEnvDisabled: true, distroNameWslPath: mockBadOutput, wantErr: true},

		"Error when pro status command fails":           {proStatusCommand: mockError, wantErr: true},
		"Error when pro status output cannot be parsed": {proStatusCommand: mockBadOutput, wantErr: true},

		"Error when /etc/os-release cannot be read":       {osRelease: mockError, wantErr: true},
		"Error whem /etc/os-release returns bad contents": {osRelease: mockBadOutput, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			system, mock := testutils.MockSystemInfo(t)
			mock.SetControlArg(testutils.ProStatusAttached)

			if tc.distroNameEnvDisabled {
				mock.WslDistroNameEnvEnabled = false
			}

			switch tc.distroNameWslPath {
			case mockOK:
			case mockError:
				mock.SetControlArg(testutils.WslpathErr)
			case mockBadOutput:
				mock.SetControlArg(testutils.WslpathBadOutput)
			default:
				require.Fail(t, "Unknown enum value for distroNameWslPath", "Value: %d", tc.distroNameWslPath)
			}

			switch tc.proStatusCommand {
			case mockOK:
			case mockError:
				mock.SetControlArg(testutils.ProStatusErr)
			case mockBadOutput:
				mock.SetControlArg(testutils.ProStatusBadJSON)
			default:
				require.Failf(t, "Unknown enum value for proStatusCommand", "Value: %d", tc.proStatusCommand)
			}

			switch tc.osRelease {
			case mockOK:
			case mockError:
				os.Remove(mock.Path("/etc/os-release"))
			case mockBadOutput:
				err := os.WriteFile(mock.Path("/etc/os-release"), []byte("This file has the wrong syntax"), 0600)
				require.NoError(t, err, "Setup: could not overwrite /etc/os-release")
			default:
				require.Failf(t, "Unknown enum value for osRelease", "Value: %d", tc.osRelease)
			}

			info, err := system.Info(ctx)
			if tc.wantErr {
				require.Error(t, err, "Expected Get() to return an error")
				return
			}
			require.NoError(t, err, "Expected Get() to return no errors")

			assert.Equal(t, "TEST_DISTRO", info.WslName, "WslName does not match expected value")
			assert.Equal(t, "ubuntu", info.Id, "Id does not match expected value")
			assert.Equal(t, "22.04", info.VersionId, "VersionId does not match expected value")
			assert.Equal(t, "Ubuntu 22.04.1 LTS", info.PrettyName, "PrettyName does not match expected value")
			assert.Equal(t, true, info.ProAttached, "ProAttached does not match expected value")
		})
	}
}

func TestLocalAppData(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		cachedCmdExe      bool
		cmdExeNotExist    bool
		cmdExeErr         bool
		wslpathErr        bool
		overrideProcMount bool

		wantErr bool
	}{
		"Success with cached cmd.exe path": {cachedCmdExe: true},

		"Success with a single 9P filesystem mount":        {overrideProcMount: true},
		"Success with multiple 9P filesystem mounts":       {overrideProcMount: true},
		"Success with multiple types of filesystem mounts": {overrideProcMount: true},

		"Error finding cmd.exe because there is no /proc/mounts":               {wantErr: true, overrideProcMount: true},
		"Error finding cmd.exe because there is no Windows FS in /proc/mounts": {wantErr: true, overrideProcMount: true},
		"Error when cmd.exe does not exist":                                    {cmdExeNotExist: true, overrideProcMount: true, wantErr: true},
		"Error on cmd.exe error":                                               {cmdExeErr: true, wantErr: true},
		"Error on wslpath error":                                               {wslpathErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			if tc.cmdExeErr {
				mock.SetControlArg(testutils.CmdExeErr)
			}
			if tc.wslpathErr {
				mock.SetControlArg(testutils.WslpathErr)
			}
			if tc.overrideProcMount {
				overrideProcMount(t, mock)
			}

			cmdExePath := mock.Path("/mnt/d/WINDOWS/system32/cmd.exe")
			if tc.cachedCmdExe {
				cmdExePath = mock.Path("/mnt/z/WINDOWS/system32/cmd.exe")
				*system.CmdExeCache() = cmdExePath
			}
			if tc.cmdExeNotExist {
				os.RemoveAll(cmdExePath)
			}

			got, err := system.LocalAppData(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected LocalAppData to return an error")
				return
			}
			require.NoError(t, err, "Expected LocalAppData to return no errors")

			// Validating CMD path
			require.Equal(t, cmdExePath, *system.CmdExeCache(), "Unexpected path for cmd.exe")

			// Validating LocalAppData
			wantSuffix := `/mnt/c/Users/TestUser/AppData/Local`
			require.True(t, strings.HasSuffix(got, wantSuffix), "Unexpected value returned by LocalAppData.\nWant suffix: %s\nGot: %s", wantSuffix, got)
		})
	}
}

func overrideProcMount(t *testing.T, mock *testutils.SystemInfoMock) {
	t.Helper()

	procMount := filepath.Join(golden.TestFixturePath(t), "proc/mounts")
	if _, err := os.Stat(procMount); err != nil {
		require.ErrorIsf(t, err, os.ErrNotExist, "Setup: could not stat %q", procMount)

		// If the file is not present, we remove the default
		t.Log("Removing default proc/mounts")
		err := os.RemoveAll(mock.Path("/proc/mounts"))
		require.NoError(t, err, "Setup: could not remove override for /proc/mounts")

		return
	}

	contents, err := os.ReadFile(procMount)
	require.NoError(t, err, "Setup: could not read override for /proc/mounts")

	err = os.WriteFile(mock.Path("/proc/mounts"), contents, 0600)
	require.NoError(t, err, "Setup: could not override /proc/mounts")
}

func TestProStatus(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		proMock  mockBehaviour
		attached bool

		wantErr bool
	}{
		"success on unattached distro": {},
		"success on attached distro":   {attached: true},

		"error on 'pro attach' returning bad output": {proMock: mockBadOutput, wantErr: true},
		"error on 'pro attach' error":                {proMock: mockError, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			switch tc.proMock {
			case mockOK:
			case mockBadOutput:
				mock.SetControlArg(testutils.ProStatusBadJSON)
			case mockError:
				mock.SetControlArg(testutils.ProStatusErr)
			default:
				require.Fail(t, "Unknown enum value for proMock", "Value: %d", tc.proMock)
			}

			if tc.attached {
				mock.SetControlArg(testutils.ProStatusAttached)
			}

			got, services, err := system.ProStatus(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected ProStatus to return an error")
				return
			}
			require.NoError(t, err, "Expected ProStatus to return no errors")

			require.Equal(t, tc.attached, got, "Unexpected return from ProStatus")
			if tc.attached {
				require.ElementsMatch(t, []string{"example-service"}, services, "Unexpected services returned from ProStatus")
			}
		})
	}
}

func TestProAttach(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		proErr bool

		wantErr bool
	}{
		"success":                     {},
		"error on 'pro attach' error": {proErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			if tc.proErr {
				mock.SetControlArg(testutils.ProAttachErr)
			}

			err := system.ProAttach(context.Background(), "1000")
			if tc.wantErr {
				require.Error(t, err, "Expected ProAttach to return an error")
				return
			}
			require.NoError(t, err, "Expected ProAttach to return no errors")
		})
	}
}

func TestProDetach(t *testing.T) {
	t.Parallel()

	type detachResult int
	const (
		detachOK detachResult = iota
		detachErrNoReason
		detachErrAlreadyDetached
		detachErrOtherReason
		detachErrCannotParseReason
	)

	testCases := map[string]struct {
		detachResult detachResult

		wantErr bool
	}{
		"success on unattached distro": {},
		"success on attached distro":   {detachResult: detachErrAlreadyDetached},

		"error on 'pro detach' returning error and no reason": {detachResult: detachErrNoReason, wantErr: true},
		"error on 'pro detach' error and some reason":         {detachResult: detachErrOtherReason, wantErr: true},
		"error on 'pro detach' error with bad JSON":           {detachResult: detachErrCannotParseReason, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			switch tc.detachResult {
			case detachOK:
			case detachErrNoReason:
				mock.SetControlArg(testutils.ProDetachErrNoReason)
			case detachErrAlreadyDetached:
				mock.SetControlArg(testutils.ProDetachErrAlreadyDetached)
			case detachErrOtherReason:
				mock.SetControlArg(testutils.ProDetachErrGeneric)
			case detachErrCannotParseReason:
				mock.SetControlArg(testutils.ProDetachBadJSON)
				mock.SetControlArg(testutils.ProDetachErrGeneric)
			default:
				require.Fail(t, "Unknown enum value for detachResult", "Value: %d", tc.detachResult)
			}

			err := system.ProDetach(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected ProStatus to return an error")
				return
			}
			require.NoError(t, err, "Expected ProStatus to return no errors")
		})
	}
}

func TestProEnablement(t *testing.T) {
	t.Parallel()

	type action int
	const (
		enable action = iota
		disable
	)

	type returns int
	const (
		ok = iota
		err
		errAlreadyDone
		errNoExplanation
		errBadJSON
	)

	testCases := map[string]struct {
		action        action
		proExeReturns returns

		wantErr bool
	}{
		"Success enabling a service":  {action: enable},
		"Success disabling a service": {action: disable},

		"Success enabling a service that was already enabled":   {action: enable, proExeReturns: errAlreadyDone},
		"Success disabling a service that was already disabled": {action: disable, proExeReturns: errAlreadyDone},

		"Error when pro enable returns an error":  {action: enable, proExeReturns: err, wantErr: true},
		"Error when pro disable returns an error": {action: disable, proExeReturns: err, wantErr: true},

		"Error when pro enable returns an unexplained error":  {action: enable, proExeReturns: errNoExplanation, wantErr: true},
		"Error when pro disable returns an unexplained error": {action: disable, proExeReturns: errNoExplanation, wantErr: true},

		"Error when pro enable returns an error with bad JSON":  {action: enable, proExeReturns: errBadJSON, wantErr: true},
		"Error when pro disable returns an error with bad JSON": {action: disable, proExeReturns: errBadJSON, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			switch tc.proExeReturns {
			case ok:
			case err:
				mock.SetControlArg(testutils.ProEnableErr)
				mock.SetControlArg(testutils.ProDisableErr)
			case errBadJSON:
				mock.SetControlArg(testutils.ProEnableErrBadJSON)
				mock.SetControlArg(testutils.ProDisableErrBadJSON)
			case errNoExplanation:
				mock.SetControlArg(testutils.ProEnableErrNoReason)
				mock.SetControlArg(testutils.ProDisableErrNoReason)
			case errAlreadyDone:
				mock.SetControlArg(testutils.ProEnableErrAlreadyEnabled)
				mock.SetControlArg(testutils.ProDisableErrAlreadyDisabled)
			default:
				require.Fail(t, "Setup: unknown enum value %v for proExeReturns", tc.proExeReturns)
			}

			var err error
			switch tc.action {
			case enable:
				err = system.ProEnablement(context.Background(), "esm-infra", true)
			case disable:
				err = system.ProEnablement(context.Background(), "esm-infra", false)
			default:
				require.Fail(t, "Setup: unknown enum value %v for afterState", tc.action)
			}

			if tc.wantErr {
				require.Error(t, err, "ProEnablement should return an error")
				return
			}
			require.NoError(t, err, "ProEnablement should return no error")
		})
	}
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithCmdExeMock(t *testing.T)  { testutils.CmdExeMock(t) }
