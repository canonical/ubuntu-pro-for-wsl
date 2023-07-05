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

	testCases := map[string]bool{
		"edu":      true,
		"onetwo12": true,

		"outer space":         false,
		"_xXProGamerXx_":      false,
		"testcase@ubuntu.com": false,
		"15€":                 false,
		"💩emoji":              false,
	}

	for username, want := range testCases {
		username := username
		want := want

		t.Run(fmt.Sprintf("test username %q", username), func(t *testing.T) {
			t.Parallel()

			got := distroinstall.UsernameIsValid(username)
			require.Equal(t, want, got, "Unexpected value for UsernameIsValid")
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
