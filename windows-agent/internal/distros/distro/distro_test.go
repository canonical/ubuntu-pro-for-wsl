package distro_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
		withGUID string

		wantErrType error
	}{
		"Registered distro":               {distro: registeredDistro},
		"Registered distro with its GUID": {distro: registeredDistro, withGUID: registeredGUID},

		// Error cases
		"Registered distro, another distro's GUID":          {distro: nonRegisteredDistro, withGUID: anotherRegisteredGUID, wantErrType: &distro.NotValidError{}},
		"Registered distro, non-matching GUID":              {distro: registeredDistro, withGUID: fakeGUID, wantErrType: &distro.NotValidError{}},
		"Non-registered distro":                             {distro: nonRegisteredDistro, wantErrType: &distro.NotValidError{}},
		"Non-registered distro, another distro's GUID":      {distro: nonRegisteredDistro, withGUID: registeredGUID, wantErrType: &distro.NotValidError{}},
		"Non-registered distro, with a non-registered GUID": {distro: nonRegisteredDistro, withGUID: fakeGUID, wantErrType: &distro.NotValidError{}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var d *distro.Distro
			var err error

			var args []distro.Option
			if tc.withGUID != "" {
				GUID, err := windows.GUIDFromString(tc.withGUID)
				require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
				args = append(args, distro.WithGUID(GUID))
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
			require.Equal(t, registeredGUID, d.GUID(), "distro.GUID should match the one it was constructed with")
			require.Equal(t, props, d.Properties, "distro.Properties should match the one it was constructed with because they were never directly modified")
		})
	}
}

func TestString(t *testing.T) {
	name, guid := testutils.RegisterDistro(t, false)

	GUID, err := windows.GUIDFromString(guid)
	require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
	d, err := distro.New(name, distro.Properties{}, t.TempDir(), distro.WithGUID(GUID))

	require.NoError(t, err, "Setup: unexpected error in distro.New")

	s := d.String()
	require.Contains(t, s, name, "String() should contain the name of the distro")
	require.Contains(t, s, guid, "String() should contain the GUID of the distro")
}

func TestIsValid(t *testing.T) {
	distro1, guid1 := testutils.RegisterDistro(t, false)
	_, guid2 := testutils.RegisterDistro(t, false)
	nonRegisteredDistro, fakeGUID := testutils.NonRegisteredDistro(t)

	testCases := map[string]struct {
		distro string
		guid   string

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

			GUID, err := windows.GUIDFromString(tc.guid)
			require.NoError(t, err, "Setup: could not parse guid %s: %v", GUID, err)
			d.GetIdentity().GUID = GUID

			got := d.IsValid()
			require.Equal(t, tc.want, got, "IsValid should return expected value")
		})
	}
}

func TestKeepAwake(t *testing.T) {
	const wslSleepDelay = 8 * time.Second

	testCases := map[string]struct {
		unregisterDistro bool
		invalidateDistro bool

		wantErr bool
	}{
		"Registered distro is kept awake": {},
		"Error on invalidated distro":     {invalidateDistro: true, wantErr: true},
		"Error on uregistered distro":     {unregisterDistro: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			distroName, _ := testutils.RegisterDistro(t, false)

			d, err := distro.New(distroName, distro.Properties{}, t.TempDir())
			require.NoError(t, err, "Setup: distro New should return no error")
			t.Cleanup(func() { d.Cleanup(context.Background()) })

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testutils.TerminateDistro(t, distroName)

			if tc.invalidateDistro {
				d.Invalidate(errors.New("setup: invalidating distro"))
			}
			if tc.unregisterDistro {
				testutils.UnregisterDistro(t, distroName)
			}

			err = d.KeepAwake(ctx)
			if tc.wantErr {
				require.Error(t, err, "KeepAwake should have returned an error")

				time.Sleep(5 * time.Second)
				state := testutils.DistroState(t, distroName)
				require.NotEqual(t, "Running", state, "distro should not run when KeepAwake is called")

				return
			}
			require.NoError(t, err, "KeepAwake should have returned no error")

			require.Eventually(t, func() bool {
				return testutils.DistroState(t, distroName) == "Running"
			}, 10*time.Second, time.Second, "distro should have started after calling keepAwake")

			time.Sleep(2 * wslSleepDelay)

			require.Equal(t, "Running", testutils.DistroState(t, distroName), "KeepAwake should have kept the distro running")

			cancel()

			require.Eventually(t, func() bool {
				return testutils.DistroState(t, distroName) == "Stopped"
			}, 2*wslSleepDelay, time.Second, "distro should have stopped after calling keepAwake due to inactivity")

		})
	}
}
