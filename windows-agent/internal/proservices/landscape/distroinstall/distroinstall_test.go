package distroinstall_test

import (
	"context"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

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
