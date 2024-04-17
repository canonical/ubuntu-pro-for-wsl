package proservices_test

import (
	"os"
	"path/filepath"
	"testing"

	ps "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/stretchr/testify/require"
)

func TestNewTLSCertificates(t *testing.T) {
	t.Parallel()
	testcases := map[string]struct {
		inexistentDestDir bool
		breakKeyFile      string

		wantErr bool
	}{
		"Success": {},

		"Error when the destination directory does not exist": {inexistentDestDir: true, wantErr: true},
		"Error when the server private key cannot be written": {breakKeyFile: "server_key.pem", wantErr: true},
		"Error when the client private key cannot be written": {breakKeyFile: "client_key.pem", wantErr: true},
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

			_, err := ps.NewTLSCertificates(dir)
			if tc.wantErr {
				require.Error(t, err, "NewTLSCertificates should have failed")
				return
			}
			require.NoError(t, err, "NewTLSCertificates failed")
		})
	}
}
