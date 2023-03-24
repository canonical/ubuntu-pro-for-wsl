package systeminfo_test

import (
	"context"
	"os"
	"strings"
	"testing"

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
		wslpathErr bool

		wantErr bool
	}{
		"success":                {},
		"error on wslpath error": {wslpathErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			if tc.wslpathErr {
				mock.SetControlArg(testutils.WslpathErr)
			}

			got, err := system.LocalAppData(context.Background())
			if tc.wantErr {
				require.Error(t, err, "Expected LocalAppData to return an error")
				return
			}
			require.NoError(t, err, "Expected LocalAppData to return no errors")

			wantSuffix := `/mnt/c/Users/TestUser/AppData/Local`
			require.True(t, strings.HasSuffix(got, wantSuffix), "Unexpected value returned by LocalAppData.\nWant suffix: %s\nGot: %s", wantSuffix, got)
		})
	}
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

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
