package database_test

import (
	"context"
	"sync"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"go.yaml.in/yaml/v3"
)

func TestSerializableDistroMarshallUnmarshall(t *testing.T) {
	t.Parallel()

	testCases := map[string]database.SerializableDistro{
		"Normal case": {
			Name: "Ubuntu",
			GUID: "{12345678-1234-1234-1234-123456789abc}",
			Properties: distro.Properties{
				DistroID:    "Ubuntu",
				VersionID:   "98.04",
				PrettyName:  "Ubuntu 98.04.0 LTS",
				ProAttached: true,
				Hostname:    "Machine98",
			},
		},
		"Escaped characters": {
			Name: "Ubuntu",
			GUID: "{12345678-1234-1234-1234-123456789abc}",
			Properties: distro.Properties{
				DistroID:    "Ubuntu",
				VersionID:   "122.04",
				PrettyName:  `Ubuntu '122.04.0 LTS "Jammiest Jellifish"`,
				ProAttached: true,
				Hostname:    "Machine122",
			},
		},
		"Control characters": {
			Name: "Ubuntu",
			GUID: "{12345678-1234-1234-1234-123456789abc}",
			Properties: distro.Properties{
				DistroID:    "Ubuntu",
				VersionID:   "122.04",
				PrettyName:  `Ubuntu 122.04.0 LTS\t (Evil\x00 character eðition)`,
				ProAttached: true,
				Hostname:    "Machine122",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			marshalled, err := yaml.Marshal(tc)
			require.NoError(t, err, "A serializableDistro should always succeed in marshalling")

			// We don't really care what the text representation is, so long as the original
			// object can be recovered. We log it here for informational purposes.
			t.Logf("%s", marshalled)

			var got database.SerializableDistro
			err = yaml.Unmarshal(marshalled, &got)
			require.NoError(t, err, "serializableDistro should be successfully unmarshalled")

			require.Equal(t, tc, got, "A Marshalled-then-Unmarshalled serializableDistro should be identical to its original version")
		})
	}
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestSerializableDistroNewDistro(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	registeredDistro, registeredGUID := wsltestutils.RegisterDistro(t, ctx, false)
	unregisteredDistro, fakeGUID := wsltestutils.NonRegisteredDistro(t)
	illFormedGUID := "{this string is not a valid GUID}"

	testCases := map[string]struct {
		distro string
		guid   string

		wantErr bool
	}{
		"Deserialize registered distro with matching GUID": {distro: registeredDistro, guid: registeredGUID},

		"Error with registered distro with non-matching GUID":       {distro: registeredDistro, guid: fakeGUID, wantErr: true},
		"Error on registered distro with ill-formed GUID":           {distro: registeredDistro, guid: illFormedGUID, wantErr: true},
		"Error on non-registered distro with a registered GUID":     {distro: unregisteredDistro, guid: registeredGUID, wantErr: true},
		"Error on non-registered distro with a non-registered GUID": {distro: unregisteredDistro, guid: fakeGUID, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := database.SerializableDistro{
				Name: tc.distro,
				GUID: tc.guid,
			}

			// This distro is never started, so no need for any global mutex
			var mu sync.Mutex

			d, err := s.NewDistro(ctx, t.TempDir(), &mu)
			if err == nil {
				defer d.Cleanup(context.Background())
			}

			if tc.wantErr {
				require.Error(t, err, "serializableDistro.New() should fail with the provided serializableDistro object")
				return
			}
			require.NoError(t, err, "serializableDistro.New() should succeed when the provided serializableDistro is valid")
		})
	}
}

func TestNewSerializableDistro(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	registeredDistro, registeredGUID := wsltestutils.RegisterDistro(t, ctx, false)

	props := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "-5.04",
		PrettyName:  "Ubuntu -5.04 (Invented Idea)",
		ProAttached: true,
		Hostname:    "NegativeMachine",
	}

	// This distro is never started, so no need for any global mutex
	var mu sync.Mutex

	d, err := distro.New(ctx, registeredDistro, props, t.TempDir(), &mu)
	require.NoError(t, err, "Setup: distro New() should return no error")

	s := database.NewSerializableDistro(d)
	require.Equal(t, registeredDistro, s.Name)
	require.Equal(t, registeredGUID, s.GUID)
	require.Equal(t, props, s.Properties)
}
