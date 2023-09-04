package tasks_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common/golden"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func TestNewLandscapeConfigure(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		wrongDistroName bool

		wantErr         bool
		wantEmptyConfig bool
	}{
		"Success enabling": {},
		"Success enabling when the computer_title key already exists": {},
		"Success disabling": {wantEmptyConfig: true},

		"Error enabling when there is no client section": {wantErr: true},
		"Error enabling when the file cannot be parsed":  {wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			conf, err := os.ReadFile(filepath.Join(golden.TestFixturePath(t), "landscape-client.conf"))
			require.NoError(t, err, "Setup: could not load config")

			t.Log(string(conf))

			const distroName = "TEST_DISTRO_NAME"
			task, err := tasks.NewLandscapeConfigure(ctx, string(conf), distroName)
			t.Log(task.Config)
			if tc.wantErr {
				require.Error(t, err, "NewLandscapeConfigure should have returned an error")
				return
			}
			require.NoError(t, err, "NewLandscapeConfigure should have succeeded")

			require.Equal(t, reflect.TypeOf(task).Name(), task.String(), "Task String() does not match the name of the task")

			if tc.wantEmptyConfig {
				require.Empty(t, task.Config, "Config was expected to be empty")
				return
			}
			require.NotEmpty(t, task.Config, "Config was not expected to be empty")

			d, err := ini.Load([]byte(task.Config))
			require.NoError(t, err, "could not load config as ini file")

			require.True(t, d.HasSection("client"), "section [client] was expected")
			require.True(t, d.Section("client").HasKey("computer_title"), "key computer_title was expected")
			require.Equal(t, distroName, d.Section("client").Key("computer_title").Value(), "key computer_title was expected to equal the distro name")
		})
	}
}
