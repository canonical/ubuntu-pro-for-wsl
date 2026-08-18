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

	dir := t.TempDir()

	cfg, err := newTLSCertificates(dir)
	require.NoError(t, err, "newTLSCertificates failed")
	require.NotNil(t, cfg, "newTLSCertificates should have returned a TLS config")

	certsDir := filepath.Join(dir, common.CertificatesDir)
	entries, err := os.ReadDir(certsDir)
	require.NoError(t, err, "could not read certificates directory")
	require.Len(t, entries, 3, "exactly three files should be published")

	wantNames := map[string]struct{}{
		common.RootCACertFileName:                               {},
		common.ClientsCertFilePrefix + common.CertificateSuffix: {},
		common.ClientsCertFilePrefix + common.KeySuffix:         {},
	}
	for _, entry := range entries {
		delete(wantNames, entry.Name())
	}
	require.Empty(t, wantNames, "not all expected publishable files were written")

	_, err = os.Stat(filepath.Join(certsDir, common.AgentCertFilePrefix+common.CertificateSuffix))
	require.True(t, os.IsNotExist(err), "agent certificate must not be written to disk")
	_, err = os.Stat(filepath.Join(certsDir, common.AgentCertFilePrefix+common.KeySuffix))
	require.True(t, os.IsNotExist(err), "agent private key must not be written to disk")
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
