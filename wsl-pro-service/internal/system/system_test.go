package system_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/golden"
	commontestutils "github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
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

		hostnameErr bool

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

		"Error when hostname cannot be obtained": {hostnameErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			system, mock := testutils.MockSystem(t)
			mock.SetControlArg(testutils.ProStatusAttached)

			if tc.distroNameEnvDisabled {
				mock.WslDistroNameEnvEnabled = false
			}

			if tc.hostnameErr {
				mock.DistroHostname = nil
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

			assert.Equal(t, "TEST_DISTRO", info.GetWslName(), "WslName does not match expected value")
			assert.Equal(t, "ubuntu", info.GetId(), "Id does not match expected value")
			assert.Equal(t, "22.04", info.GetVersionId(), "VersionId does not match expected value")
			assert.Equal(t, "Ubuntu 22.04.1 LTS", info.GetPrettyName(), "PrettyName does not match expected value")
			assert.Equal(t, "TEST_DISTRO_HOSTNAME", info.GetHostname(), "Hostname does not match expected value")
			assert.True(t, info.GetProAttached(), "ProAttached does not match expected value")
		})
	}
}

func TestUserProfileDir(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		cachedCmdExe      bool
		cmdExeNotExist    bool
		cmdExeErr         bool
		wslpathErr        bool
		wslpathBadOutput  bool
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
		"Error when wslpath returns a bad path":                                {wslpathBadOutput: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystem(t)
			if tc.cmdExeErr {
				mock.SetControlArg(testutils.CmdExeErr)
			}
			if tc.wslpathErr {
				mock.SetControlArg(testutils.WslpathErr)
			}
			if tc.wslpathBadOutput {
				mock.SetControlArg(testutils.WslpathBadOutput)
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

			got, err := system.UserProfileDir(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected UserProfile to return an error")
				return
			}
			require.NoError(t, err, "Expected UserProfile to return no errors")

			// Validating CMD path
			require.Equal(t, cmdExePath, *system.CmdExeCache(), "Unexpected path for cmd.exe")

			// Validating UserProfile
			wantSuffix := `/mnt/d/Users/TestUser`
			require.True(t, strings.HasSuffix(got, wantSuffix), "Unexpected value returned by UserProfileDir.\nWant suffix: %s\nGot: %s", wantSuffix, got)
		})
	}
}

func overrideProcMount(t *testing.T, mock *testutils.SystemMock) {
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

			system, mock := testutils.MockSystem(t)
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

			got, err := system.ProStatus(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected ProStatus to return an error")
				return
			}
			require.NoError(t, err, "Expected ProStatus to return no errors")

			require.Equal(t, tc.attached, got, "Unexpected return from ProStatus")
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

			system, mock := testutils.MockSystem(t)
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

			system, mock := testutils.MockSystem(t)
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

func TestLandscapeEnable(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakWriteConfig     bool
		breakLandscapeConfig bool
		breakWSLPath         bool

		wantErr bool
	}{
		"Success":                                    {},
		"Success overriding computer_title":          {},
		"Success overriding the SSL certficate path": {},

		"Error when the file cannot be parsed":                   {wantErr: true},
		"Error when the config file cannot be written":           {breakWriteConfig: true, wantErr: true},
		"Error when the landscape-config command fails":          {breakLandscapeConfig: true, wantErr: true},
		"Error when failing to override the SSL certficate path": {breakWSLPath: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			s, mock := testutils.MockSystem(t)

			if tc.breakWriteConfig {
				path := mock.Path(system.LandscapeConfigPath)
				commontestutils.ReplaceFileWithDir(t, path, "Setup: could not create directory to interfere with config file creation")
			}

			if tc.breakLandscapeConfig {
				mock.SetControlArg(testutils.LandscapeEnableErr)
			}

			if tc.breakWSLPath {
				mock.SetControlArg(testutils.WslpathErr)
			}

			config, err := os.ReadFile(filepath.Join(golden.TestFixturePath(t), "landscape.conf"))
			require.NoError(t, err, "Setup: could not load fixture")

			err = s.LandscapeEnable(ctx, string(config), "landscapeUID1234")
			if tc.wantErr {
				require.Error(t, err, "LandscapeEnable should have returned an error")
				return
			}
			require.NoError(t, err, "LandscapeEnable should have succeeded")

			exeProof := s.Path("/.landscape-enabled")
			require.FileExists(t, exeProof, "Landscape executable never ran")
			out, err := os.ReadFile(exeProof)
			require.NoErrorf(t, err, "could not read file %q", exeProof)

			// We mock the filesystem, and the mocked filesystem root is not the same between
			// runs, so the golden file would never match. This is the solution:
			got := strings.ReplaceAll(string(out), mock.FsRoot, "${FILESYSTEM_ROOT}")

			want := golden.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Landscape executable did not receive the right config")
		})
	}
}

func TestWindowsHostAddress(t *testing.T) {
	t.Parallel()

	type fileState = int
	const (
		fileOK fileState = iota
		fileNotExist
		fileBroken
		fileIPbroken
		fileIPisLoopback
	)

	// copyFile is a helper that copies the appropriate version of a fixture to the desired destination.
	copyFile := func(t *testing.T, state fileState, from, to string) {
		t.Helper()

		var suffix string
		switch state {
		case fileNotExist:
			err := os.RemoveAll(to)
			require.NoError(t, err, "Setup: could not remove file %s", to)
			return
		case fileOK:
			suffix = ".good"
		case fileBroken:
			suffix = ".bad"
		case fileIPbroken:
			suffix = ".bad-ip"
		case fileIPisLoopback:
			suffix = ".loopback"
		}

		from = from + suffix
		out, err := os.ReadFile(from)
		require.NoErrorf(t, err, "Setup: could not read file %s", from)
		err = os.WriteFile(to, out, 0400)
		require.NoErrorf(t, err, "Setup: could not write file %s", to)
	}

	// These are the addresses hard-coded on the fixtures labelled as "good"
	const (
		localhost   = "127.0.0.1"
		nameserver  = "172.22.16.1"
		degaultGway = "172.25.32.1"
	)

	testCases := map[string]struct {
		networkNotNAT bool
		breakWslInfo  bool

		etcResolv    fileState
		procNetRoute fileState

		want    string
		wantErr bool
	}{
		"Success without NAT":                          {networkNotNAT: true, want: localhost},
		"Success with NAT, nameserver is not loopback": {want: nameserver},
		"Success with NAT, nameserver is loopback":     {etcResolv: fileIPisLoopback, want: degaultGway},

		// WSL info errors
		"Error when wslinfo returns an error": {breakWslInfo: true, wantErr: true},

		// NAT errors with broken /etc/resolv.conf
		"Error with NAT when /etc/resolv.conf does not exist":       {etcResolv: fileNotExist, wantErr: true},
		"Error with NAT when /etc/resolv.conf is ill-formed":        {etcResolv: fileBroken, wantErr: true},
		"Error with NAT when /etc/resolv.conf has an ill-formed IP": {etcResolv: fileIPbroken, wantErr: true},

		// NAT errors with loopback nameserver and broken /proc/net/route
		"Error with NAT when /proc/net/route does not exist":       {etcResolv: fileIPisLoopback, procNetRoute: fileNotExist, wantErr: true},
		"Error with NAT when /proc/net/route is ill-formed":        {etcResolv: fileIPisLoopback, procNetRoute: fileBroken, wantErr: true},
		"Error with NAT when /proc/net/route has an ill-formed IP": {etcResolv: fileIPisLoopback, procNetRoute: fileIPbroken, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			sys, mock := testutils.MockSystem(t)

			if tc.breakWslInfo {
				mock.SetControlArg(testutils.WslInfoErr)
			}
			if !tc.networkNotNAT {
				mock.SetControlArg(testutils.WslInfoIsNAT)
			}

			copyFile(t, tc.etcResolv, filepath.Join(golden.TestFamilyPath(t), "etc-resolv.conf"), mock.Path("/etc/resolv.conf"))
			copyFile(t, tc.procNetRoute, filepath.Join(golden.TestFamilyPath(t), "proc-net-route"), mock.Path("/proc/net/route"))

			got, err := sys.WindowsHostAddress(ctx)
			if tc.wantErr {
				require.Error(t, err, "WindowsHostAddress should return an error")
				return
			}
			require.NoError(t, err, "WindowsHostAddress should return no error")
			require.Equal(t, tc.want, got.String(), "Wrong IP returned by WindowsHostAddress")
		})
	}
}

func TestLandscapeDisable(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakLandscapeConfig bool

		wantErr bool
	}{
		"Success": {},

		"Error when the landscape-config command fails": {breakLandscapeConfig: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			s, mock := testutils.MockSystem(t)

			if tc.breakLandscapeConfig {
				mock.SetControlArg(testutils.LandscapeDisableErr)
			}

			err := s.LandscapeDisable(ctx)
			if tc.wantErr {
				require.Error(t, err, "LandscapeDisable should have returned an error")
				return
			}
			require.NoError(t, err, "LandscapeDisable should have succeeded")

			require.FileExists(t, s.Path("/.landscape-disabled"), "Landscape executable never ran")
		})
	}
}

func TestWithProMock(t *testing.T)             { testutils.ProMock(t) }
func TestWithLandscapeConfigMock(t *testing.T) { testutils.LandscapeConfigMock(t) }
func TestWithWslPathMock(t *testing.T)         { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T)         { testutils.WslInfoMock(t) }
func TestWithCmdExeMock(t *testing.T)          { testutils.CmdExeMock(t) }
