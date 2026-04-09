package proservices

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestNewTLSCertificates(t *testing.T) {
	t.Parallel()
	testcases := map[string]struct {
		inexistentDestDir bool
		breakKeyFile      string

		wantErr bool
	}{
		"Success": {},

		"Error when the destination directory does not exist":  {inexistentDestDir: true, wantErr: true},
		"Error when the agent private key cannot be written":   {breakKeyFile: common.AgentCertFilePrefix + common.KeySuffix, wantErr: true},
		"Error when the clients private key cannot be written": {breakKeyFile: common.ClientsCertFilePrefix + common.KeySuffix, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tc.inexistentDestDir {
				dir = filepath.Join(dir, "inexistent")
			}

			if tc.breakKeyFile != "" {
				err := os.MkdirAll(filepath.Join(dir, tc.breakKeyFile), 0700)
				require.NoError(t, err, "Setup: could not write directory that should break %s", tc.breakKeyFile)
			}

			c, err := newTLSCertificates(dir)
			if tc.wantErr {
				require.Error(t, err, "NewTLSCertificates should have failed")
				return
			}
			require.NoError(t, err, "NewTLSCertificates failed")
			require.NotEmpty(t, c, "NewTLSCertificates should have returned a non-empty value")
		})
	}
}

func TestNewInstanceHook(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		breakConfig bool
		proToken    string
		lcape       string
		props       distro.Properties

		wantErr   bool
		taskCount int
	}{
		"Success":                       {proToken: "token", lcape: "[client]", taskCount: 2},
		"Success with a pro token only": {proToken: "token", taskCount: 1},
		"Success with no tasks":         {taskCount: 0},
		"No tasks when the instance is already pro attached": {proToken: "token", props: distro.Properties{ProAttached: true}, wantErr: false, taskCount: 0},
		"Error when the config cannot be loaded":             {breakConfig: true, wantErr: true},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Set up the config.
			privateDir := t.TempDir()
			fileData := struct {
				Landscape    map[string]string
				Subscription map[string]string
			}{
				Subscription: make(map[string]string),
				Landscape:    make(map[string]string),
			}
			if tc.proToken != "" {
				fileData.Subscription["user"] = tc.proToken
			}
			if tc.lcape != "" {
				fileData.Landscape["config"] = tc.lcape
			}
			if tc.breakConfig {
				require.NoError(t, os.MkdirAll(filepath.Join(privateDir, "config"), 0700), "Setup: could not write directory that should break config file")
			} else {
				b, err := yaml.Marshal(fileData)
				require.NoError(t, err, "Setup: could not marshal config data")
				require.NoError(t, os.WriteFile(filepath.Join(privateDir, "config"), b, 0600), "Setup: could not write config file")
			}
			ctx := t.Context()
			conf := config.New(ctx, privateDir)
			tsks, err := newInstanceTasks(conf, tc.props)
			if tc.wantErr {
				require.Error(t, err, "NewInstanceTasks should have failed")
				return
			}
			require.NoError(t, err, "NewInstanceTasks failed")
			require.Len(t, tsks, tc.taskCount, "NewInstanceTasks returned unexpected number of tasks")
		})
	}
}
