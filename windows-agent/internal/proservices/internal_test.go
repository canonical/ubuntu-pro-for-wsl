package proservices

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestNewTLSCertificates(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		// breakCertificatesDir places a regular file where the certificates
		// directory should be, so sub-scoping the custodian must fail.
		breakCertificatesDir bool
		// breakPublishableFile places a directory where a certificate file should
		// be written, so the custodian write must fail.
		breakPublishableFile bool
		// leaveStaleFile seeds leftover certificate material from a previous run.
		leaveStaleFile bool

		wantErr bool
	}{
		"Success removes stale certificate files": {leaveStaleFile: true},

		"Error when the certificates directory cannot be created": {breakCertificatesDir: true, wantErr: true},
		"Error when a publishable file cannot be written":         {breakPublishableFile: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			if tc.breakCertificatesDir {
				err := os.WriteFile(filepath.Join(dir, common.CertificatesDir), []byte{}, 0600)
				require.NoError(t, err, "Setup: could not create the file that should break the certificates directory")
			}
			if tc.leaveStaleFile {
				staleDir := filepath.Join(dir, common.CertificatesDir)
				err := os.MkdirAll(staleDir, 0700)
				require.NoError(t, err, "Setup: could not create the stale certificates directory")
				err = os.WriteFile(filepath.Join(staleDir, "stale.pem"), []byte("stale"), 0600)
				require.NoError(t, err, "Setup: could not create stale certificate file")
			}
			if tc.breakPublishableFile {
				err := os.MkdirAll(filepath.Join(dir, common.CertificatesDir, common.RootCACertFileName), 0700)
				require.NoError(t, err, "Setup: could not create the directory that should break the publishable file")
			}

			custodian, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer custodian.Close()

			certsCust, err := custodian.Subdir(common.CertificatesDir)
			if tc.breakCertificatesDir {
				require.Error(t, err, "sub-scoping the custodian onto a regular file should fail")
				return
			}
			require.NoError(t, err, "Setup: could not sub-scope certificates custodian")
			defer certsCust.Close()

			cfg, err := newTLSCertificates(certsCust)
			if tc.wantErr {
				require.Error(t, err, "newTLSCertificates should have failed")
				return
			}
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

			if tc.leaveStaleFile {
				_, err = os.Stat(filepath.Join(certsDir, "stale.pem"))
				require.True(t, os.IsNotExist(err), "stale certificate file should have been removed")
			}

			_, err = os.Stat(filepath.Join(certsDir, common.AgentCertFilePrefix+common.CertificateSuffix))
			require.True(t, os.IsNotExist(err), "agent certificate must not be written to disk")
			_, err = os.Stat(filepath.Join(certsDir, common.AgentCertFilePrefix+common.KeySuffix))
			require.True(t, os.IsNotExist(err), "agent private key must not be written to disk")
		})
	}
}

func TestNewTLSCertificatesFilesystemFailures(t *testing.T) {
	t.Parallel()
	// Permission-based failures work differently (or not at all) on Windows, so
	// we verify these branches on Unix-like systems only.
	if runtime.GOOS == "windows" {
		t.Skip("permission-based filesystem failures are not portable to Windows")
	}

	testcases := map[string]struct {
		breakReadDir bool
		breakRemove  bool
	}{
		"Error when the certificates directory cannot be read":  {breakReadDir: true},
		"Error when a stale certificate file cannot be removed": {breakRemove: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			certsDir := filepath.Join(dir, common.CertificatesDir)
			require.NoError(t, os.MkdirAll(certsDir, 0700), "Setup: could not create certificates directory")

			// Open the custodians before the permission sabotage: sub-scoping requires
			// reading the certificates directory, which the sabotage intentionally breaks.
			custodian, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer custodian.Close()

			certsCust, err := custodian.Subdir(common.CertificatesDir)
			require.NoError(t, err, "Setup: could not sub-scope certificates custodian")
			defer certsCust.Close()

			if tc.breakReadDir {
				// Remove read permission so the custodian's ReadDir fails.
				//nolint:gosec // G302 - test setup needs directory execute permission.
				require.NoError(t, os.Chmod(certsDir, 0100), "Setup: could not remove read permission")
				defer func() {
					//nolint:gosec // G302 - test teardown restores directory permissions.
					require.NoError(t, os.Chmod(certsDir, 0700), "Teardown: could not restore permissions")
				}()
			}

			if tc.breakRemove {
				require.NoError(t, os.WriteFile(filepath.Join(certsDir, "stale.pem"), []byte("stale"), 0600), "Setup: could not create stale certificate file")
				// Remove write permission from the directory so the custodian's Remove fails.
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(certsDir, 0500), "Setup: could not remove write permission")
				defer func() {
					//nolint:gosec // G302 - test teardown restores directory permissions.
					require.NoError(t, os.Chmod(certsDir, 0700), "Teardown: could not restore permissions")
				}()
			}

			_, err = newTLSCertificates(certsCust)
			require.Error(t, err, "newTLSCertificates should have failed")
		})
	}
}

// newTLSCertificatesWithFailingPKI is a newTLSCertificatesOption that makes PKI
// generation fail, exercising the error path without mutating global state.
func newTLSCertificatesWithFailingPKI() newTLSCertificatesOption {
	return func(o *newTLSCertificatesOpts) {
		o.generatePKI = func() (certs.PKI, error) {
			return certs.PKI{}, errors.New("injected PKI failure")
		}
	}
}

func TestNewTLSCertificatesPKIFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	custodian, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer custodian.Close()

	certsCust, err := custodian.Subdir(common.CertificatesDir)
	require.NoError(t, err, "Setup: could not sub-scope certificates custodian")
	defer certsCust.Close()

	_, err = newTLSCertificates(certsCust, newTLSCertificatesWithFailingPKI())
	require.Error(t, err, "newTLSCertificates should have failed")
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
