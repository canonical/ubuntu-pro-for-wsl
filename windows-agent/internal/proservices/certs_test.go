package proservices_test

import (
	"os"

	"path/filepath"
	"testing"

	ps "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/stretchr/testify/require"
)

func TestNewTLSCertificates(t *testing.T) {

	testcases := map[string]struct {
		breakDestDir bool
		breakServer  bool

		wantErr bool
	}{
		"Success": {},
		"Error when the destination directory cannot be written into": {breakDestDir: true, wantErr: true},
		"Error when the server private key cannot be written":         {breakServer: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.breakDestDir {
				dir = filepath.Join(dir, "inexistent")
			}

			if tc.breakServer {
				err := os.MkdirAll(filepath.Join(dir, "server_key.pem"), 0700)
				require.NoError(t, err, "Setup: could not write directory that should break writing the server key")
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
