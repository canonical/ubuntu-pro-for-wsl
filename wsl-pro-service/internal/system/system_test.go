package system_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		systemLandscapeConfigFile string
	}{
		"Return a new system": {},

		"Ignore errors when the Landscape config validation failed (only warnings)": {systemLandscapeConfigFile: "invalid_ini.conf"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := &testutils.SystemMock{
				FsRoot: testutils.MockFilesystemRoot(t),
			}

			if tc.systemLandscapeConfigFile != "" {
				config, err := os.ReadFile(filepath.Join("testdata", "landscape.conf.d", tc.systemLandscapeConfigFile))
				require.NoError(t, err, "Setup: could not load fixture")
				err = os.MkdirAll(filepath.Dir(mock.Path(system.LandscapeConfigPath)), 0700)
				require.NoError(t, err, "Setup: could not create Landscape config dir")
				err = os.WriteFile(mock.Path(system.LandscapeConfigPath), config, 0600)
				require.NoError(t, err, "Setup: could not write Landscape system config file")
			}

			s := system.New(system.WithTestBackend(mock))

			require.NotNil(t, s, "New should return a system object")
		})
	}
}

func TestInfo(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// This causes Get to look at the Windows' path for /
		badWslDistroName bool
		proStatusCommand mockBehaviour
		osRelease        mockBehaviour

		hostnameErr bool

		wantErr bool
	}{
		"Success": {},

		"Error when WslDistroName fails": {badWslDistroName: true, wantErr: true},

		"Error when pro status command fails":           {proStatusCommand: mockError, wantErr: true},
		"Error when pro status output cannot be parsed": {proStatusCommand: mockBadOutput, wantErr: true},

		"Error when /etc/os-release cannot be read":       {osRelease: mockError, wantErr: true},
		"Error whem /etc/os-release returns bad contents": {osRelease: mockBadOutput, wantErr: true},

		"Error when hostname cannot be obtained": {hostnameErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			system, mock := testutils.MockSystem(t)
			mock.SetControlArg(testutils.ProStatusAttached)

			if tc.badWslDistroName {
				mock.SetControlArg(testutils.WslpathErr)
				mock.WslDistroNameEnvEnabled = false
			}

			if tc.hostnameErr {
				mock.DistroHostname = nil
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
				require.Error(t, err, "Expected Info() to return an error")
				return
			}
			require.NoError(t, err, "Expected Info() to return no errors")

			assert.Equal(t, "TEST_DISTRO", info.GetWslName(), "WslName does not match expected value")
			assert.Equal(t, "ubuntu", info.GetId(), "Id does not match expected value")
			assert.Equal(t, "22.04", info.GetVersionId(), "VersionId does not match expected value")
			assert.Equal(t, "Ubuntu 22.04.1 LTS", info.GetPrettyName(), "PrettyName does not match expected value")
			assert.Equal(t, "TEST_DISTRO_HOSTNAME", info.GetHostname(), "Hostname does not match expected value")
			assert.True(t, info.GetProAttached(), "ProAttached does not match expected value")
		})
	}
}

func TestWslDistroName(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// This causes Get to look at the Windows' path for /
		distroNameEnvDisabled bool

		// The path of "/" according to the Windows host
		distroNameWslPath mockBehaviour

		wantErr bool
	}{
		"Success reading from WSL_DISTRO_NAME": {},
		"Success using wslpath":                {distroNameEnvDisabled: true},

		"Error when WSL_DISTRO_NAME is empty and wslpath fails":            {distroNameEnvDisabled: true, distroNameWslPath: mockError, wantErr: true},
		"Error when WSL_DISTRO_NAME is empty and wslpath returns bad text": {distroNameEnvDisabled: true, distroNameWslPath: mockBadOutput, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			system, mock := testutils.MockSystem(t)

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

			got, err := system.WslDistroName(ctx)
			if tc.wantErr {
				require.Error(t, err, "Expected WslDistroName() to return an error")
				return
			}
			require.NoError(t, err, "Expected WslDistroName() to return no errors")
			assert.Equal(t, "TEST_DISTRO", got, "WslDistroName does not match expected value")

			// Test the cache: second call should not call wslpath
			mock.SetControlArg(testutils.WslpathErr)
			got, err = system.WslDistroName(ctx)
			require.NoError(t, err, "WslDistroName should return no error as the cache should be used")
			assert.Equal(t, "TEST_DISTRO", got, "WslDistroName does not match expected value in second call")
		})
	}
}

func TestUserProfileDir(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		cachedCmdExe           bool
		cmdExeNotExist         bool
		cmdExeErr              bool
		cmdEncodingErr         bool
		emptyUserprofileEnvVar bool
		wslpathErr             bool
		wslpathBadOutput       bool
		overrideProcMount      bool

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
		"Error on cmd.exe output encoding wrong":                               {cmdEncodingErr: true, wantErr: true},
		"Error when UserProfile env var is empty":                              {emptyUserprofileEnvVar: true, wantErr: true},
		"Error on wslpath error":                                               {wslpathErr: true, wantErr: true},
		"Error when wslpath returns a bad path":                                {wslpathBadOutput: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystem(t)
			if tc.cmdExeErr {
				mock.SetControlArg(testutils.CmdExeErr)
			}
			if tc.cmdEncodingErr {
				mock.SetControlArg(testutils.CmdExeEncodingErr)
			}
			if tc.emptyUserprofileEnvVar {
				mock.SetControlArg(testutils.EmptyUserprofileEnvVar)
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
				require.Error(t, err, "Expected UserProfile to return an error, but returned %s intead", got)
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

	procMount := filepath.Join(commontestutils.TestFixturePath(t), "proc/mounts")
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
		landscapeConfigFile string

		breakWriteConfigDir     bool
		breakWriteConfig        bool
		breakLandscapeConfigCmd bool
		breakWSLPath            bool
		breakWSLDistroName      bool
		noLandscapeGroup        bool

		wantErr bool
	}{
		"Appends any required fields - no ssl":                      {landscapeConfigFile: "minimal.conf"},
		"Transform Windows SSL certificate path":                    {landscapeConfigFile: "windows_ssl_only.conf"},
		"Transform Windows SSL certificate path with forward slash": {landscapeConfigFile: "windows_ssl_only_forward_slash.conf"},
		"Refresh computer_title if changed":                         {landscapeConfigFile: "old_computer_title.conf"},

		"Regular with additional keys":            {landscapeConfigFile: "regular.conf"},
		"Do not modify other sections and keys":   {landscapeConfigFile: "regular_with_extra_keys.conf"},
		"Reformat Landscape config to proper ini": {landscapeConfigFile: "regular_with_weird_format.conf"},

		"Rerun landscape even without modifications": {landscapeConfigFile: "no_change_needed.conf"},

		"Error when the new config cannot be parsed":             {landscapeConfigFile: "invalid_ini.conf", wantErr: true},
		"Error when the new config do not have client section":   {landscapeConfigFile: "no_client_section.conf", wantErr: true},
		"Error when the config directory cannot be created":      {breakWriteConfigDir: true, wantErr: true},
		"Error when the config file cannot be renamed":           {breakWriteConfig: true, wantErr: true},
		"Error when the landscape-config command fails":          {breakLandscapeConfigCmd: true, wantErr: true},
		"Error when failing to override the SSL certficate path": {breakWSLPath: true, wantErr: true},
		"Error when the can not get WSL Distro name":             {breakWSLDistroName: true, wantErr: true},
		"Error when the Landscape user does not exist":           {noLandscapeGroup: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			s, mock := testutils.MockSystem(t)

			if tc.landscapeConfigFile == "" {
				tc.landscapeConfigFile = "regular.conf"
			}

			if tc.breakWriteConfigDir {
				path := filepath.Dir(mock.Path(system.LandscapeConfigPath))
				require.NoError(t, os.RemoveAll(path), "Setup: could not remove config directory")
				require.NoError(t, os.WriteFile(path, nil, 0600), "Setup: could not create file to interfere with config directory creation")
			}

			if tc.breakWriteConfig {
				path := mock.Path(system.LandscapeConfigPath)
				commontestutils.ReplaceFileWithDir(t, path, "Setup: could not create directory to interfere with config file creation")
			}

			if tc.breakLandscapeConfigCmd {
				mock.SetControlArg(testutils.LandscapeEnableErr)
			}

			if tc.breakWSLPath {
				mock.SetControlArg(testutils.WslpathErr)
			}

			if tc.breakWSLDistroName {
				mock.SetControlArg(testutils.WslpathErr)
				mock.WslDistroNameEnvEnabled = false
			}

			if tc.noLandscapeGroup {
				mock.LandscapeGroupGID = ""
			}

			config, err := os.ReadFile(filepath.Join("testdata", "landscape.conf.d", tc.landscapeConfigFile))
			require.NoError(t, err, "Setup: could not load fixture")

			err = s.LandscapeEnable(ctx, string(config))
			if tc.wantErr {
				require.Error(t, err, "LandscapeEnable should have returned an error")
				return
			}
			require.NoError(t, err, "LandscapeEnable should have succeeded")

			// landscape --config has been executed
			exeProof := s.Path("/.landscape-enabled")
			require.FileExists(t, exeProof, "Landscape executable never ran")

			// Landscape config file has been written
			configFileContent, err := os.ReadFile(s.Path(system.LandscapeConfigPath))
			require.NoErrorf(t, err, "could not read config file %q", s.Path(system.LandscapeConfigPath))

			// We mock the filesystem, and the mocked filesystem root is not the same between
			// runs, so the golden file would never match. This is the solution:
			got := strings.ReplaceAll(string(configFileContent), mock.FsRoot, "${FILESYSTEM_ROOT}")

			want := commontestutils.LoadWithUpdateFromGolden(t, got)
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
		defaultGway = "172.25.32.1"
	)

	testCases := map[string]struct {
		networkNotNAT bool
		breakWslInfo  bool

		procNetRoute fileState

		want    string
		wantErr bool
	}{
		"Without NAT": {networkNotNAT: true, want: localhost},
		"With NAT":    {want: defaultGway},

		// WSL info errors
		"Error when wslinfo returns an error": {breakWslInfo: true, wantErr: true},

		// NAT errors with loopback nameserver and broken /proc/net/route
		"Error with NAT when /proc/net/route does not exist":       {procNetRoute: fileNotExist, wantErr: true},
		"Error with NAT when /proc/net/route is ill-formed":        {procNetRoute: fileBroken, wantErr: true},
		"Error with NAT when /proc/net/route has an ill-formed IP": {procNetRoute: fileIPbroken, wantErr: true},
	}

	for name, tc := range testCases {
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

			copyFile(t, tc.procNetRoute, filepath.Join(commontestutils.TestFamilyPath(t), "proc-net-route"), mock.Path("/proc/net/route"))

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

func TestEnsureValidLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		systemLandscapeConfigFile string

		breakWriteConfig        bool
		breakLandscapeConfigCmd bool
		breakWSLPath            bool
		breakWSLDistroName      bool
		noLandscapeGroup        bool

		wantNoLandscapeConfig    bool
		wantNoLandscapeConfigCmd bool
		wantErr                  bool
	}{
		"Appends any required fields - no ssl":                      {systemLandscapeConfigFile: "minimal.conf"},
		"Transform Windows SSL certificate path":                    {systemLandscapeConfigFile: "windows_ssl_only.conf"},
		"Transform Windows SSL certificate path with forward slash": {systemLandscapeConfigFile: "windows_ssl_only_forward_slash.conf"},
		"Do not transform Windows SSL certificate empty path":       {systemLandscapeConfigFile: "windows_ssl_empty.conf", wantNoLandscapeConfigCmd: true},
		"Refresh computer_title if changed":                         {systemLandscapeConfigFile: "old_computer_title.conf"},

		"Regular with additional keys":            {systemLandscapeConfigFile: "regular.conf"},
		"Do not modify other sections and keys":   {systemLandscapeConfigFile: "regular_with_extra_keys.conf"},
		"Reformat Landscape config to proper ini": {systemLandscapeConfigFile: "regular_with_weird_format.conf"},

		"Do not rerun landscape without modifications":                             {systemLandscapeConfigFile: "no_change_needed.conf", wantNoLandscapeConfigCmd: true},
		"Do not rerun landscape due whitespace changes":                            {systemLandscapeConfigFile: "no_change_due_spaces.conf", wantNoLandscapeConfigCmd: true},
		"No Landscape configuration means no landscape command nor config created": {systemLandscapeConfigFile: "-", wantNoLandscapeConfigCmd: true, wantNoLandscapeConfig: true},

		"Error when the config file cannot be read":              {breakWriteConfig: true, wantErr: true},
		"Error when the new config cannot be parsed":             {systemLandscapeConfigFile: "invalid_ini.conf", wantErr: true},
		"Error when the new config do not have client section":   {systemLandscapeConfigFile: "no_client_section.conf", wantErr: true},
		"Error when the landscape-config command fails":          {breakLandscapeConfigCmd: true, wantErr: true},
		"Error when failing to override the SSL certficate path": {breakWSLPath: true, wantErr: true},
		"Error when the can not get WSL Distro name":             {breakWSLDistroName: true, wantErr: true},
		"Error when the landscape user does not exist":           {noLandscapeGroup: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			s, mock := testutils.MockSystem(t)

			if tc.systemLandscapeConfigFile == "" {
				tc.systemLandscapeConfigFile = "regular.conf"
			}

			if tc.breakLandscapeConfigCmd {
				mock.SetControlArg(testutils.LandscapeEnableErr)
			}

			if tc.breakWSLPath {
				mock.SetControlArg(testutils.WslpathErr)
			}

			if tc.breakWSLDistroName {
				mock.SetControlArg(testutils.WslpathErr)
				mock.WslDistroNameEnvEnabled = false
			}

			if tc.noLandscapeGroup {
				mock.LandscapeGroupGID = ""
			}

			if tc.systemLandscapeConfigFile != "-" {
				if tc.breakWriteConfig {
					path := mock.Path(system.LandscapeConfigPath)
					commontestutils.ReplaceFileWithDir(t, path, "Setup: could not create directory to interfere with config file creation")
				} else {
					config, err := os.ReadFile(filepath.Join("testdata", "landscape.conf.d", tc.systemLandscapeConfigFile))
					require.NoError(t, err, "Setup: could not load fixture")
					err = os.MkdirAll(filepath.Dir(s.Path(system.LandscapeConfigPath)), 0700)
					require.NoError(t, err, "Setup: could not create Landscape config dir")
					err = os.WriteFile(s.Path(system.LandscapeConfigPath), config, 0600)
					require.NoError(t, err, "Setup: could not write Landscape system config file")
				}
			}

			err := s.EnsureValidLandscapeConfig(ctx)
			if tc.wantErr {
				require.Error(t, err, "EnsureValidLandscapeConfig should have returned an error")
				return
			}
			require.NoError(t, err, "EnsureValidLandscapeConfig should have succeeded")

			// Landscape --config has been executed
			exeProof := s.Path("/.landscape-enabled")
			if tc.wantNoLandscapeConfigCmd {
				require.NoFileExists(t, exeProof, "Landscape executable should not be ran")
			} else {
				require.FileExists(t, exeProof, "Landscape executable never ran")
			}

			// No Landscape config should be on the system.
			if tc.wantNoLandscapeConfig {
				require.NoFileExists(t, s.Path(system.LandscapeConfigPath), "Landscape system config not on disk")
				return
			}

			// Landscape config file has been kept or modified
			configFileContent, err := os.ReadFile(s.Path(system.LandscapeConfigPath))
			require.NoErrorf(t, err, "could not read config file %q", s.Path(system.LandscapeConfigPath))

			// We mock the filesystem, and the mocked filesystem root is not the same between
			// runs, so the golden file would never match. This is the solution:
			got := strings.ReplaceAll(string(configFileContent), mock.FsRoot, "${FILESYSTEM_ROOT}")

			want := commontestutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Landscape executable did not receive the right config")
		})
	}
}

func TestRealBackend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	b := system.RealBackend{}

	// Asserting generated commands
	//
	// Note that we cannot test the cmd.Path directly as it will depend on the install location,
	// so we only test the base of the path.

	pro := b.ProExecutable(ctx, "arg1", "arg2")
	assertBasePath(t, "pro", pro.Path, "ProExecutable did not return the expected command")
	assert.Equal(t, []string{"pro", "arg1", "arg2"}, pro.Args, "ProExecutable did not return the expected arguments")

	lpe := b.LandscapeConfigExecutable(ctx, "arg1", "arg2")
	assertBasePath(t, "landscape-config", lpe.Path, "LandscapeConfigExecutable did not return the expected command")
	assert.Equal(t, []string{"landscape-config", "arg1", "arg2"}, lpe.Args, "LandscapeConfigExecutable did not return the expected arguments")

	wpath := b.WslpathExecutable(ctx, "arg1", "arg2")
	assertBasePath(t, "wslpath", wpath.Path, "WslpathExecutable did not return the expected command")
	assert.Equal(t, []string{"wslpath", "arg1", "arg2"}, wpath.Args, "WslpathExecutable did not return the expected arguments")

	winfo := b.WslinfoExecutable(ctx, "arg1", "arg2")
	assertBasePath(t, "wslinfo", winfo.Path, "WslinfoExecutable did not return the expected command")
	assert.Equal(t, []string{"wslinfo", "arg1", "arg2"}, winfo.Args, "WslinfoExecutable did not return the expected arguments")

	cmd := b.CmdExe(ctx, "/mnt/c/WINDOWS/whatever/cmd.exe", "arg1", "arg2")
	assert.Equal(t, "/mnt/c/WINDOWS/whatever", cmd.Dir, "CmdExe did not set the expected directory")
	assert.Equal(t, "/mnt/c/WINDOWS/whatever/cmd.exe", cmd.Path, "CmdExe did not return the expected command")
	assert.Equal(t, []string{"/mnt/c/WINDOWS/whatever/cmd.exe", "arg1", "arg2"}, cmd.Args, "CmdExe did not return the expected arguments")
}

// Asserts that the base of got is equal to wantBase, and if not, it fails the test with a message.
func assertBasePath(t *testing.T, wantBase, got, msg string) {
	t.Helper()
	base := filepath.Base(got)
	assert.Equalf(t, wantBase, base, "Mismatch in base path.\n%s", msg)
}

func TestWithProMock(t *testing.T)             { testutils.ProMock(t) }
func TestWithLandscapeConfigMock(t *testing.T) { testutils.LandscapeConfigMock(t) }
func TestWithWslPathMock(t *testing.T)         { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T)         { testutils.WslInfoMock(t) }
