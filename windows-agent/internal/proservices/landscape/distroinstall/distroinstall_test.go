package distroinstall_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestUsernameIsValid(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		username string
		want     bool
	}{
		"Accept only letters":               {username: "edu", want: true},
		"Accept numbers":                    {username: "onetwo12", want: true},
		"Accept allowed special characters": {username: "johndoe_", want: true},

		"Reject spaces":                   {username: "outer space", want: false},
		"Reject initial non-letter":       {username: "_xXProGamerXx_", want: false},
		"Reject special characters":       {username: "testcase@ubuntu.com", want: false},
		"Reject other special characters": {username: "15€", want: false},
		"Reject emojis":                   {username: "💩", want: false},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := distroinstall.UsernameIsValid(tc.username)
			require.Equal(t, tc.want, got, "Unexpected value for UsernameIsValid")
		})
	}
}

func TestCreateUser(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	type mockErr int
	const (
		mockErrNone mockErr = iota
		isRegisteredMockErr
		addUserMockErr
		addUserToGroupsMockErr
		removePasswordMockErr
		getUserIDMockErr
		getUserIDMockBadOutput
	)

	testCases := map[string]struct {
		invalidUserName        bool
		invalidFullName        bool
		skipDistroRegistration bool

		mockErr mockErr

		wantErr bool
	}{
		"Success": {},
		"Success when the user's full name is not valid": {invalidFullName: true},

		"Error when the distro is not registered": {skipDistroRegistration: true, wantErr: true},
		"Error when the username is not valid":    {invalidUserName: true, wantErr: true},

		// Mock errors
		"Error when the distro registered check fails":       {mockErr: isRegisteredMockErr, wantErr: true},
		"Error when adduser returns an error":                {mockErr: addUserMockErr, wantErr: true},
		"Error when usermod returns an error":                {mockErr: addUserToGroupsMockErr, wantErr: true},
		"Error when passwd returns an error":                 {mockErr: removePasswordMockErr, wantErr: true},
		"Error when getUserID returns an error":              {mockErr: getUserIDMockErr, wantErr: true},
		"Error when getUserID returns a non-numerical value": {mockErr: getUserIDMockBadOutput, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			applyMock := func() {}
			if wsl.MockAvailable() {
				t.Parallel()
				m := wslmock.New()
				//nolint:nolintlint // False positive only when mock is disabled
				applyMock = func() { //nolint:staticcheck
					m.OpenLxssKeyError = (tc.mockErr == isRegisteredMockErr)
				}
				defer m.ResetErrors()
				ctx = wsl.WithMock(ctx, m)
			} else if tc.mockErr != mockErrNone {
				t.Skip("This test is only available when the gowslmock is enabled")
			}

			var distroName string
			if tc.skipDistroRegistration {
				distroName = wsltestutils.RandomDistroName(t)
			} else {
				distroName, _ = wsltestutils.RegisterDistro(t, ctx, true)
			}
			d := wsl.NewDistro(ctx, distroName)

			username := "johndoe"
			if tc.invalidUserName {
				username = "johndoe && echo 'code injection is fun!' && exit 5"
			}

			switch tc.mockErr {
			case addUserMockErr:
				username = "add_user_command_error"
			case addUserToGroupsMockErr:
				username = "add_user_to_groups_command_error"
			case removePasswordMockErr:
				username = "remove_password_command_error"
			case getUserIDMockErr:
				username = "get_user_id_command_error"
			case getUserIDMockBadOutput:
				username = "get_user_id_command_bad_output"
			}

			userFullName := "John Doe"
			if tc.invalidFullName {
				userFullName = "'JohnDoe,5,7777777,123456789,Hobby:I inject data into the GECOS' string && exit 5"
			}

			applyMock()

			uid, err := distroinstall.CreateUser(ctx, d, username, userFullName)
			if tc.wantErr {
				require.Error(t, err, "CreateUser should return an error")
				return
			}
			require.NoError(t, err, "CreateUser should return no error")

			_, err = distroinstall.CreateUser(ctx, d, username, userFullName)
			require.Error(t, err, "CreateUser should return error when the user already exists")

			if wsl.MockAvailable() {
				return
			}

			out, err := d.Command(ctx, fmt.Sprintf(`id %q`, username)).Output()
			require.NoError(t, err, "user should have been created")

			require.Contains(t, string(out), fmt.Sprintf("uid=%d(%s)", uid, username), "`id USER` should contain the UID returned by CreateUser")
			require.Contains(t, string(out), "sudo", "user should be in the sudoers group")

			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			//nolint:gosec // Both distroName and userName are validared in CreateUser
			_, err = exec.CommandContext(ctx, "wsl", "-d", distroName, "-u", username, "--", "sudo", "echo", "hello").Output()
			require.NoError(t, err, "user should be able to login without a password")
		})
	}
}

func TestInstallFromExecutable(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		distroName  string
		mockErr     bool
		skipInstall bool

		wantErr          bool
		wantExecNotFound bool
	}{
		"Success": {distroName: "Ubuntu-22.04"},

		// We have nothing against debian, this simply fails the injection safety check without failing wsl --install
		"Error when the distro has not a valid name": {distroName: "Debian", wantErr: true},
		"Error when the distro registration fails":   {distroName: "Ubuntu-22.04", mockErr: true, wantErr: true},
		"Error when the executable is not found":     {distroName: "Ubuntu-04.04" /*404 :)*/, skipInstall: true, wantErr: true, wantExecNotFound: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()

				m := wslmock.New()
				m.WslRegisterDistributionError = tc.mockErr
				defer m.ResetErrors()

				ctx = wsl.WithMock(ctx, m)
			} else if tc.mockErr {
				t.Skip("This test is only available with the mock enabled")
			}

			if !tc.skipInstall {
				err := wsl.Install(ctx, tc.distroName)
				require.NoError(t, err, "Setup: Install should return no errors")
			}

			d := wsl.NewDistro(ctx, tc.distroName)
			defer d.Uninstall(ctx) //nolint:errcheck // We don't care

			err := distroinstall.InstallFromExecutable(ctx, d)
			if tc.wantErr {
				require.Error(t, err, "InstallFromExecutable should return an error")
				if tc.wantExecNotFound {
					var target *distroinstall.CommandNotFoundError
					require.ErrorAs(t, err, &target, "InstallFromExecutable should return a CommandNotFoundError")
				}
				return
			}
			require.NoError(t, err, "InstallFromExecutable should return no errors")

			r, err := d.IsRegistered()
			require.NoError(t, err, "IsRegistered should return no errors")
			require.True(t, r, "InstallFromExecutable should have registered the distro")

			err = distroinstall.InstallFromExecutable(ctx, d)
			require.Error(t, err, "InstallFromExecutable should return an error when the distro is already installed")
		})
	}
}
