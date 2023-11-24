package proservices_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/registrywatcher/registry"
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
		breakConfig      bool
		breakMkDir       bool
		breakNewDistroDB bool

		wantErr bool
	}{
		"Success when the subscription stays empty":               {},
		"Success when the config cannot check if it is read-only": {breakConfig: true},

		"Error when Manager cannot create its cache dir":  {breakMkDir: true, wantErr: true},
		"Error when database cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir := t.TempDir()

			reg := registry.NewMock()
			reg.KeyExists = true

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

			s, err := proservices.New(ctx, proservices.WithCacheDir(dir), proservices.WithRegistry(reg))
			if err == nil {
				defer s.Stop(ctx)
			}

			if tc.wantErr {
				require.Error(t, err, "New should return an error")
				return
			}
			require.NoError(t, err, "New should return no error")
		})
	}
}

func TestRegisterGRPCServices(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ps, err := proservices.New(ctx, proservices.WithCacheDir(t.TempDir()), proservices.WithRegistry(registry.NewMock()))
	require.NoError(t, err, "Setup: New should return no error")
	defer ps.Stop(ctx)

	server := ps.RegisterGRPCServices(context.Background())
	info := server.GetServiceInfo()

	_, ok := info["agentapi.UI"]
	require.True(t, ok, "UI service should be registered after calling RegisterGRPCServices")

	_, ok = info["agentapi.WSLInstance"]
	require.True(t, ok, "WSLInstance service should be registered after calling RegisterGRPCServices")

	require.Lenf(t, info, 2, "Info should contain exactly two elements")
}
