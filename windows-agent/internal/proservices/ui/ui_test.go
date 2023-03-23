package ui_test

import (
	"context"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialtasks"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"github.com/stretchr/testify/require"
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

func TestAttachProInitial(t *testing.T) {
	t.Parallel()

	info := agentapi.AttachInfo{Token: "funny_token"}
	testCases := map[string]struct {
		token string

		initialErrs bool
		distroErrs  bool
	}{
		"Initial tasks succeeds, but distro fails": {token: info.Token, initialErrs: false, distroErrs: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			initTasks, err := initialtasks.New(dir)
			require.NoError(t, err, "Setup: initial tasks New() should return no error")
			serv := ui.New(context.Background(), db, initTasks)

			_, err = serv.ProAttach(context.Background(), &info)
			if tc.distroErrs {
				require.Error(t, err, "Adding the task to existing distros should fail.")
			} else {
				require.NoError(t, err, "Adding the task to existing distros should succeed.")
			}

			// Yet the initial tasks must have been populated.
			it := initTasks.All()
			if tc.initialErrs {
				require.Error(t, err, "Adding to initial tasks should fail.")
			} else {
				require.Equal(t, len(it), 1, "Only one task should have been added")
				require.Equal(t, it[0], tasks.AttachPro{Token: tc.token})
			}
		})
	}
}
