package proservices_test

import (
	"context"
	"crypto/sha512"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		configIsReadOnly bool

		breakConfig      bool
		breakMkDir       bool
		breakNewDistroDB bool

		subscriptionFileUpToDate bool
		subscriptionFileOutdated bool

		wantErr bool
	}{
		"Success":                           {},
		"Success with a read-only registry": {configIsReadOnly: true},
		"Success when the config cannot check if it is read-only": {breakConfig: true},

		"Success when the subscription is not new": {subscriptionFileUpToDate: true},
		"Success when the subscription is new":     {subscriptionFileOutdated: true},

		"Error when Manager cannot create its cache dir":  {breakMkDir: true, wantErr: true},
		"Error when database cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir := t.TempDir()

			const testToken = "TestToken123"
			testTokenCS := sha512.Sum512([]byte(testToken))

			reg := registry.NewMock()
			reg.KeyExists = true
			reg.UbuntuProData["ProTokenUser"] = testToken
			if tc.breakConfig {
				reg.Errors = registry.MockErrOnCreateKey
			}

			subscriptionChecksumFile := filepath.Join(dir, "subscription.csum")
			if tc.subscriptionFileUpToDate {
				err := os.WriteFile(subscriptionChecksumFile, testTokenCS[:], 0600)
				require.NoError(t, err, "Setup: could not write subscription checksum file")
			} else if tc.subscriptionFileOutdated {
				err := os.WriteFile(subscriptionChecksumFile, []byte("OLD_CHECKSUM"), 0600)
				require.NoError(t, err, "Setup: could not write subscription checksum file")
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
			require.Eventually(t, func() bool {
				out, err := os.ReadFile(subscriptionChecksumFile)
				if err != nil {
					t.Logf("Could not read subscription checksum file: %v", err)
					return false
				}
				if slices.Equal(out, testTokenCS[:]) {
					return true
				}
				return false
			}, time.Second, 100*time.Millisecond, "Subscription checksum file was never updated")
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
