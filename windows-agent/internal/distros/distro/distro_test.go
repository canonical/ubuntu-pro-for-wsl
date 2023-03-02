package distro_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	registeredDistro, registeredGUID := testutils.RegisterDistro(t, false)
	_, anotherRegisteredGUID := testutils.RegisterDistro(t, false)
	nonRegisteredDistro, fakeGUID := testutils.NonRegisteredDistro(t)

	props := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "100.04",
		PrettyName:  "Ubuntu 100.04.0 LTS",
		ProAttached: true,
	}

	testCases := map[string]struct {
		distro   string
		withGUID windows.GUID

		wantErrType error
	}{
		"Registered distro":               {distro: registeredDistro},
		"Registered distro with its GUID": {distro: registeredDistro, withGUID: registeredGUID},

		// Error cases
		"Registered distro, another distro's GUID":          {distro: nonRegisteredDistro, withGUID: anotherRegisteredGUID, wantErrType: &distro.NotExistError{}},
		"Registered distro, non-matching GUID":              {distro: registeredDistro, withGUID: fakeGUID, wantErrType: &distro.NotExistError{}},
		"Non-registered distro":                             {distro: nonRegisteredDistro, wantErrType: &distro.NotExistError{}},
		"Non-registered distro, another distro's GUID":      {distro: nonRegisteredDistro, withGUID: registeredGUID, wantErrType: &distro.NotExistError{}},
		"Non-registered distro, with a non-registered GUID": {distro: nonRegisteredDistro, withGUID: fakeGUID, wantErrType: &distro.NotExistError{}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var d *distro.Distro
			var err error

			var args []distro.Option
			nilGUID := windows.GUID{}
			if tc.withGUID != nilGUID {
				args = append(args, distro.WithGUID(tc.withGUID))
			}

			d, err = distro.New(tc.distro, props, t.TempDir(), args...)
			if err == nil {
				defer d.Cleanup(context.Background())
			}
			if tc.wantErrType != nil {
				require.Error(t, err, "New() should have returned an error")
				require.ErrorIsf(t, err, tc.wantErrType, "New() should have returned an error of type %T", tc.wantErrType)
				return
			}

			require.NoError(t, err, "New() should have returned no error")
			require.Equal(t, tc.distro, d.Name(), "distro.Name should match the one it was constructed with")
			require.Equal(t, registeredGUID.String(), d.GUID().String(), "distro.GUID should match the one it was constructed with")
			require.Equal(t, props, d.Properties, "distro.Properties should match the one it was constructed with because they were never directly modified")
		})
	}
}

func TestString(t *testing.T) {
	name, guid := testutils.RegisterDistro(t, false)
	d, err := distro.New(name, distro.Properties{}, t.TempDir(), distro.WithGUID(guid))
	require.NoError(t, err, "Setup: unexpected error in distro.New")

	s := d.String()
	require.Contains(t, s, name, "String() should contain the name of the distro")
	require.Contains(t, s, strings.ToLower(guid.String()), "String() should contain the GUID of the distro")
}

func TestIsValid(t *testing.T) {
	distro1, guid1 := testutils.RegisterDistro(t, false)
	_, guid2 := testutils.RegisterDistro(t, false)
	nonRegisteredDistro, fakeGUID := testutils.NonRegisteredDistro(t)

	testCases := map[string]struct {
		distro string
		guid   windows.GUID

		want bool
	}{
		"registered distro with matching GUID": {distro: distro1, guid: guid1, want: true},

		// Invalid cases
		"registered distro with different, another distro's GUID": {distro: distro1, guid: guid2, want: false},
		"registered distro with different, fake GUID":             {distro: distro1, guid: fakeGUID, want: false},
		"non-registered distro, registered distro's GUID":         {distro: nonRegisteredDistro, guid: guid1, want: false},
		"non-registered distro, non-registered distro's GUID":     {distro: nonRegisteredDistro, guid: fakeGUID, want: false},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Create an always valid distro
			d, err := distro.New(distro1, distro.Properties{}, t.TempDir())
			require.NoError(t, err, "Setup: distro New() should return no errors")

			// Change values and assert on IsValid
			d.GetIdentity().Name = tc.distro
			d.GetIdentity().GUID = tc.guid

			got := d.IsValid()
			require.Equal(t, tc.want, got, "IsValid should return expected value")
		})
	}
}
