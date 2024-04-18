package proservices

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
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
