package ui_test

import (
	"context"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	t.Parallel()

	dir := t.TempDir()
	db, err := database.New(ctx, dir, nil)
	require.NoError(t, err, "Setup: empty database New() should return no error")
	defer db.Close(ctx)

	conf, err := config.New(ctx, config.WithRegistry(registry.NewMock()))
	require.NoError(t, err, "Setup: could not initialize Config")

	_ = ui.New(context.Background(), conf, db)
}

// Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
//
//nolint:tparallel
func TestAttachPro(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, _ := testutils.RegisterDistro(t, ctx, false)
	distro2, _ := testutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		distros          []string
		token            string
		registryReadOnly bool

		wantErr bool
	}{
		"No panic due empty token":          {token: ""},
		"Success with an empty database":    {token: "funny_token"},
		"Success with a non-empty database": {token: "whatever_token", distros: []string{distro1, distro2}},

		"Error due to no write permission on token": {registryReadOnly: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			defer db.Close(ctx)

			// Populate the database
			for i := range tc.distros {
				d, err := db.GetDistroAndUpdateProperties(context.Background(), tc.distros[i], distro.Properties{})
				require.NoError(t, err, "Setup: could not add %q to database", tc.distros[i])
				defer d.Cleanup(ctx)
			}

			m := registry.NewMock()
			m.KeyIsReadOnly = tc.registryReadOnly
			m.KeyExists = true
			m.UbuntuProData["ProToken"] = "OLD_TOKEN"

			conf, err := config.New(ctx, config.WithRegistry(m))
			require.NoError(t, err, "Setup: could not initialize Config")

			serv := ui.New(context.Background(), conf, db)

			info := agentapi.ProAttachInfo{Token: tc.token}
			_, err = serv.ApplyProToken(context.Background(), &info)
			if tc.wantErr {
				require.Error(t, err, "Unexpected success in ApplyProToken")
				return
			}
			require.NoError(t, err, "Adding the task to existing distros should succeed.")

			// Could it be nice to retrieve the distro's pending tasks?
			token, err := conf.ProToken(ctx)
			require.NoError(t, err, "conf.ProToken should return no error")
			require.Equal(t, tc.token, token, "mismatch between submitted and retrieved tokens")
		})
	}
}
