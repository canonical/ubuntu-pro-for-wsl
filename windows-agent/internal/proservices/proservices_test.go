package proservices_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakMkDir       bool
		breakNewDistroDB bool

		wantErr bool
	}{
		"Success": {},

		"Error when Manager cannot create its cache dir":  {breakMkDir: true, wantErr: true},
		"Error when distroDB cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			if tc.breakMkDir {
				dir = filepath.Join(dir, "proservices")
				err := os.WriteFile(dir, []byte("♫♪ Never gonna give you up ♫♪"), 0600)
				require.NoError(t, err, "Setup: could not write file where proservices wants to put a dir")
			}

			if tc.breakNewDistroDB {
				dbFile := filepath.Join(dir, consts.DatabaseFileName)
				err := os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: could not write directory where database wants to put a file")
			}

			_, err := proservices.New(context.Background(), proservices.WithCacheDir(dir))
			if tc.wantErr {
				require.Error(t, err, "New should return an error when there is a problem with its dir")
				return
			}
			require.NoError(t, err, "New should return no error")
		})
	}
}
