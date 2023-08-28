package proservices_test

import (
	"context"
	"crypto/sha512"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
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

	type subscription = string
	const (
		tokenA    = "Token123"
		tokenB    = "TokenXYZ"
		tokenNone = ""
	)

	testCases := map[string]struct {
		configIsReadOnly bool

		breakConfig      bool
		breakMkDir       bool
		breakNewDistroDB bool

		newSubscription subscription
		oldSubscription subscription
		checksumIsEmpty bool

		wantErr bool
	}{
		"Success when the subscription stays empty":               {},
		"Success with a read-only registry":                       {configIsReadOnly: true},
		"Success when the config cannot check if it is read-only": {breakConfig: true},

		"Success when the subscription stays the same":     {oldSubscription: tokenA, newSubscription: tokenA},
		"Success when the subscription changes tokens":     {oldSubscription: tokenA, newSubscription: tokenB},
		"Success when the subscription changes from empty": {oldSubscription: tokenNone, newSubscription: tokenA},
		"Success when the subscription changes to empty":   {oldSubscription: tokenA, newSubscription: tokenNone},

		"Success when the subscription was an empty file (subscribed)":     {checksumIsEmpty: true, newSubscription: tokenA},
		"Success when the subscription was an empty file (non-subscribed)": {checksumIsEmpty: true, newSubscription: tokenNone},

		"Error when Manager cannot create its cache dir":  {breakMkDir: true, wantErr: true},
		"Error when database cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir := t.TempDir()

			subscriptionChecksumFile := filepath.Join(dir, "subscription.csum")
			if tc.checksumIsEmpty {
				err := os.WriteFile(subscriptionChecksumFile, []byte{}, 0600)
				require.NoError(t, err, "Setup: could not write empty checksum file")
			} else if tc.oldSubscription != "" {
				oldChecksum := sha512.Sum512([]byte(tc.oldSubscription))
				err := os.WriteFile(subscriptionChecksumFile, oldChecksum[:], 0600)
				require.NoError(t, err, "Setup: could not write subscription checksum file")
			}

			reg := registry.NewMock()
			reg.KeyExists = true
			reg.UbuntuProData["ProTokenUser"] = tc.newSubscription
			if tc.breakConfig {
				reg.Errors = registry.MockErrOnCreateKey
			}

			if tc.configIsReadOnly {
				reg.KeyExists = true
				reg.KeyIsReadOnly = true
			}

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

			// Subscriptions are updated asyncronously
			time.Sleep(time.Second)

			out, err := os.ReadFile(subscriptionChecksumFile)
			if tc.newSubscription == "" {
				require.ErrorIs(t, err, fs.ErrNotExist, "The subscription file should have been removed.")
				return
			}
			require.NoError(t, err, "Could not read subscription checksum file: %v", err)

			newChecksum := sha512.Sum512([]byte(tc.newSubscription))
			require.Equal(t, out, newChecksum[:], "Checksum does no match the new subscription's")
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
