package distroinstall_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
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
		tc := tc
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

	testCases := map[string]struct {
		invalidUserName        bool
		invalidFullName        bool
		skipDistroRegistration bool

		addUserMockErr      bool
		isRegisteredMockErr bool

		wantErr bool
	}{
		"Success": {},
		"Success when the user's full name is not valid": {invalidFullName: true},

		"Error when the distro is not registered":      {skipDistroRegistration: true, wantErr: true},
		"Error when the distro registered check fails": {isRegisteredMockErr: true, wantErr: true},
		"Error when the username is not valid":         {invalidUserName: true, wantErr: true},
		"Error when useradd returns an error":          {addUserMockErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			applyMock := func() {}
			if wsl.MockAvailable() {
				t.Parallel()
				m := wslmock.New()
				//nolint:nolintlint // False positive only when mock is disabled
				applyMock = func() { //nolint:staticcheck
					m.OpenLxssKeyError = tc.isRegisteredMockErr
				}
				defer m.ResetErrors()
				ctx = wsl.WithMock(ctx, m)
			} else if tc.addUserMockErr || tc.isRegisteredMockErr {
				t.Skip("This test is only available when the gowslmock is enabled")
			}

			var distroName string
			if tc.skipDistroRegistration {
				distroName = testutils.RandomDistroName(t)
			} else {
				distroName, _ = testutils.RegisterDistro(t, ctx, true)
			}
			d := gowsl.NewDistro(ctx, distroName)

			username := "johndoe"
			if tc.invalidUserName {
				username = "johndoe && echo 'code injection is fun!' && exit 5"
			}
			if tc.addUserMockErr {
				username = "mockerror"
			}

			userFullName := "John Doe"
			if tc.invalidFullName {
				userFullName = "'JohnDoe,5,7777777,123456789,Hobby:I inject data into the GECOS' string && exit 5"
			}

			applyMock()

			err := distroinstall.CreateUser(ctx, d, username, userFullName, 1000)
			if tc.wantErr {
				require.Error(t, err, "CreateUser should return an error")
				return
			}
			require.NoError(t, err, "CreateUser should return no error")

			if wsl.MockAvailable() {
				return
			}

			err = d.Command(ctx, fmt.Sprintf(`test -d "/home/%s"`, username)).Run()
			require.NoError(t, err, "home directory for newly created user should exist")
		})
	}
}

func TestInstallFromExecutable(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		distroName string
		mockErr    bool

		wantErr bool
	}{
		"Success": {distroName: "Ubuntu-22.04"},

		// We have nothing against debian, this simply fails the injection safety check without failing wsl --install
		"Error when the distro has not a valid name": {distroName: "Debian", wantErr: true},
		"Error when the distro registration fails":   {distroName: "Ubuntu-22.04", mockErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				m := wslmock.New()
				m.WslRegisterDistributionError = tc.mockErr
				defer m.ResetErrors()

				ctx = wsl.WithMock(ctx, m)
			} else if tc.mockErr {
				t.Skip("This test is only available with the mock enabled")
			}

			err := wsl.Install(ctx, tc.distroName)
			require.NoError(t, err, "Setup: Install should return no errors")

			d := wsl.NewDistro(ctx, tc.distroName)
			defer d.Uninstall(ctx) //nolint:errcheck // We don't care

			err = distroinstall.InstallFromExecutable(ctx, d)
			if tc.wantErr {
				require.Error(t, err, "InstallFromExecutable should return an error")
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
