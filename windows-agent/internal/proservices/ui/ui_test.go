package ui_test

import (
	"context"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialtasks"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
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
	initTasks, err := initialtasks.New(dir)
	require.NoError(t, err, "Setup: initial tasks New() should return no error")

	_ = ui.New(context.Background(), db, initTasks)
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
		token string

		distros []string
	}{
		"No panic due empty token":          {token: ""},
		"Success with an empty database":    {token: "funny_token"},
		"Success with a non-empty database": {token: "whatever_token", distros: []string{distro1, distro2}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			// Populate the database
			for i := range tc.distros {
				_, err := db.GetDistroAndUpdateProperties(context.Background(), tc.distros[i], distro.Properties{})
				require.NoError(t, err, "Setup: could not add %q to database", tc.distros[i])
			}

			initTasks, err := initialtasks.New(dir)
			require.NoError(t, err, "Setup: initial tasks New() should return no error")
			serv := ui.New(context.Background(), db, initTasks)

			info := agentapi.ProAttachInfo{Token: tc.token}
			_, err = serv.ApplyProToken(context.Background(), &info)
			require.NoError(t, err, "Adding the task to existing distros should succeed.")
			// Could it be nice to retrieve the distro's pending tasks?

			it := initTasks.All()
			require.ElementsMatch(t, it, []tasks.ProAttachment{{Token: tc.token}}, "Only one task should have been added")
		})
	}
}
