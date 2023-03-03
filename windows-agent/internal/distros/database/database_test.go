package database_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type dbDirState int

const (
	emptyDbDir dbDirState = iota
	goodDbFile
	badDbDir
	badDbFile
	badDbFileContents
)

//nolint: tparallel
// Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestNew(t *testing.T) {
	distro, guid := testutils.RegisterDistro(t, false)

	testCases := map[string]struct {
		dirState dbDirState

		wantDistros []string
		wantErr     bool
	}{
		"Success on no pre-exisiting database file": {dirState: emptyDbDir, wantDistros: []string{}},
		"Success at loading distro from database":   {dirState: goodDbFile, wantDistros: []string{distro}},

		"Error with syntax error in database file":             {dirState: badDbFileContents, wantErr: true},
		"Error due to database file exists but cannot be read": {dirState: badDbFile, wantErr: true},
		"Error because it cannot create a database dir":        {dirState: badDbDir, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dbDir := t.TempDir()
			switch tc.dirState {
			case badDbDir:
				dbDir = filepath.Join(dbDir, "database")
				err := os.WriteFile(dbDir, []byte("I am here to interfere"), 0600)
				require.NoError(t, err, "Setup: could not write file where the database dir will go")
			case badDbFile:
				err := os.MkdirAll(filepath.Join(dbDir, consts.DatabaseFileName), 0600)
				require.NoError(t, err, "Setup: could not create folder where database file is supposed to go")
			case badDbFileContents:
				err := os.WriteFile(filepath.Join(dbDir, consts.DatabaseFileName), []byte("\tThis is not\nvalid yaml"), 0600)
				require.NoError(t, err, "Setup: could not write wrong database file")
			case goodDbFile:
				databaseFromTemplate(t, dbDir, distroID{distro, guid})
			}

			db, err := database.New(dbDir, nil)
			if tc.wantErr {
				require.Error(t, err, "New() should have returned an error")
				return
			}
			require.NoError(t, err, "New() should have returned no error")

			distros := db.DistroNames()
			require.ElementsMatch(t, tc.wantDistros, distros, "database should contain all the registered distros read from file")
		})
	}
}

//nolint: tparallel
// Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestDatabaseGet(t *testing.T) {
	registeredDistroInDB, registeredGUID := testutils.RegisterDistro(t, false)
	registeredDistroNotInDB, _ := testutils.RegisterDistro(t, false)

	nonRegisteredDistroNotInDB, _ := testutils.NonRegisteredDistro(t)
	nonRegisteredDistroInDB, oldGUID := testutils.RegisterDistro(t, false)

	databaseDir := t.TempDir()
	databaseFromTemplate(t, databaseDir,
		distroID{registeredDistroInDB, registeredGUID},
		distroID{nonRegisteredDistroInDB, oldGUID})

	db, err := database.New(databaseDir, nil)
	require.NoError(t, err, "Setup: New() should return no error")

	// Unregister the distro now, so that it's in the db object but not on system properly.
	testutils.UnregisterDistro(t, nonRegisteredDistroInDB)

	testCases := map[string]struct {
		distroName string

		wantNotFound bool
	}{
		"Get a registered distro in database":          {distroName: registeredDistroInDB},
		"Get an unregistered distro still in database": {distroName: nonRegisteredDistroInDB},

		"Cannot get a registered distro not present in the database":         {distroName: registeredDistroNotInDB, wantNotFound: true},
		"Cannot get a distro that is neither registered nor in the database": {distroName: nonRegisteredDistroNotInDB, wantNotFound: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			d, found := db.Get(tc.distroName)
			if tc.wantNotFound {
				require.False(t, found, "The second return value of Get(distro) should be false when asked for a distro not in the database")
				return
			}
			require.True(t, found, "The second return value of Get(distro) should be true when asked for a distro in the database")
			require.NotNil(t, d, "The first return value of Get(distro) should return a *Distro when asked for a distro in the database")

			require.Equal(t, d.Name(), tc.distroName, "The distro returned by Get should match the one in the database")
		})
	}
}

//nolint: tparallel
// Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestDatabaseDump(t *testing.T) {
	distro1, guid1 := testutils.RegisterDistro(t, false)
	distro2, guid2 := testutils.RegisterDistro(t, false)

	// Ensuring lexicographical ordering
	if strings.ToLower(distro1) > strings.ToLower(distro2) {
		distro1, distro2 = distro2, distro1
		guid1, guid2 = guid2, guid1
	}

	testCases := map[string]struct {
		dirState dbDirState
		emptyDB  bool

		wantErr bool
	}{
		"Success with a regular database":       {dirState: goodDbFile},
		"Success with an empty DB":              {dirState: goodDbFile, emptyDB: true},
		"Success writing on an empty directory": {dirState: emptyDbDir},

		"Error when it cannot write the dump to file": {dirState: badDbFile, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dbDir := t.TempDir()

			if !tc.emptyDB {
				databaseFromTemplate(t, dbDir, distroID{distro1, guid1}, distroID{distro2, guid2})
			}

			db, err := database.New(dbDir, nil)
			require.NoError(t, err, "Setup: empty database should be created without issue")

			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)
			switch tc.dirState {
			case badDbFile:
				err := os.RemoveAll(dbFile)
				require.NoError(t, err, "Setup: could not remove database dump")
				err = os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: could not create directory to interfere with database dump")
			case goodDbFile:
				// generateDatabaseFile already generated it
			case emptyDbDir:
				err := os.RemoveAll(dbFile)
				require.NoError(t, err, "Setup: could not remove pre-existing database dump")
			default:
				require.FailNow(t, "Setup: test case not implemented")
			}

			err = db.Dump()
			if tc.wantErr {
				require.Error(t, err, "Dump() should return an error when the database file (or its directory) is not valid")
				return
			}
			require.NoError(t, err, "Dump() should return no error when the database file and its directory are both valid")

			dump, err := os.ReadFile(filepath.Join(dbDir, consts.DatabaseFileName))
			require.NoError(t, err, "The database dump should be readable after calling Dump()")

			t.Logf("Generated dump:\n%s", dump)

			sd := newStructuredDump(t, dump)

			if tc.emptyDB {
				require.Empty(t, len(sd.data), "Database dump should contain no distros")
			} else {
				require.Equal(t, 2, len(sd.data), "Database dump should contain exactly two distros")

				idx1 := slices.IndexFunc(sd.data, func(s database.SerializableDistro) bool { return s.Name == distro1 })
				idx2 := slices.IndexFunc(sd.data, func(s database.SerializableDistro) bool { return s.Name == distro2 })

				require.NotEqualf(t, -1, idx1, "Database dump should contain distro1 (%s). Dump:\n%s", distro1, dump)
				require.NotEqualf(t, -1, idx2, "Database dump should contain distro2 (%s). Dump:\n%s", distro2, dump)

				require.Equal(t, sd.data[idx1].GUID, guid1, "Database dump GUID for distro1 should match the one it was constructed with. Dump:\n%s", dump)
				require.Equal(t, sd.data[idx2].GUID, guid2, "Database dump GUID for distro2 should match the one it was constructed with. Dump:\n%s", dump)
			}

			// Anonymizing
			sd.anonymise(t)

			// Testing against and optionally updating golden file
			want := testutils.LoadWithUpdateFromGoldenYAML(t, sd.data)
			require.Equal(t, want, sd.data, "Database dump should match expected format")
		})
	}
}

func TestGetDistroAndUpdateProperties(t *testing.T) {
	var distroInDB, distroNotInDB, reRegisteredDistro, nonRegisteredDistro string
	var guids map[string]string

	// Scope to avoid leaking guid variables
	{
		var guid1, guid2, guid3, guid4 string

		distroInDB, guid1 = testutils.RegisterDistro(t, false)
		distroNotInDB, guid2 = testutils.RegisterDistro(t, false)
		reRegisteredDistro, guid3 = testutils.RegisterDistro(t, false)
		nonRegisteredDistro, guid4 = testutils.NonRegisteredDistro(t)

		guids = map[string]string{
			distroInDB:          guid1,
			distroNotInDB:       guid2,
			reRegisteredDistro:  guid3,
			nonRegisteredDistro: guid4,
		}
	}

	props := map[string]distro.Properties{
		distroInDB: {
			DistroID:    "SuperUbuntu",
			VersionID:   "122.04",
			PrettyName:  "Ubuntu 122.04 LTS (Jolly Jellyfish)",
			ProAttached: false,
		},
		distroNotInDB: {
			DistroID:    "HyperUbuntu",
			VersionID:   "222.04",
			PrettyName:  "Ubuntu 122.04 LTS (Joker Jellyfish)",
			ProAttached: false,
		},
		reRegisteredDistro: {
			DistroID:    "Ubuntu",
			VersionID:   "22.04",
			PrettyName:  "Ubuntu 22.04 LTS (Jammy Jellyfish)",
			ProAttached: true,
		},
	}

	type searchResult = int
	const (
		fullHit searchResult = iota
		hitUnregisteredDistro
		hitAndRefreshProps
		missedAndAdded
	)

	testCases := map[string]struct {
		distroName   string
		props        distro.Properties
		breakDBbDump bool

		want                searchResult
		wantDbDumpRefreshed bool
		wantErr             bool
		wantErrType         error
	}{
		"Distro exists in database and properties match it": {distroName: distroInDB, props: props[distroInDB], want: fullHit},

		// Refresh/update database handling
		"Distro exists in database, with different properties updates the stored db": {distroName: distroInDB, props: props[distroNotInDB], want: hitAndRefreshProps, wantDbDumpRefreshed: true},
		"Distro exists in database, but no longer valid updates the stored db":       {distroName: reRegisteredDistro, props: props[reRegisteredDistro], want: hitUnregisteredDistro, wantDbDumpRefreshed: true},
		"Distro is not in database, we add it and update the stored db":              {distroName: distroNotInDB, props: props[distroNotInDB], want: missedAndAdded, wantDbDumpRefreshed: true},

		"Error on distro not in database and we do not add it ": {distroName: nonRegisteredDistro, wantErr: true, wantErrType: &distro.NotExistError{}},
		"Error on database refresh failing":                     {distroName: distroInDB, props: props[distroNotInDB], breakDBbDump: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			dbDir := t.TempDir()
			databaseFromTemplate(t, dbDir,
				distroID{distroInDB, guids[distroInDB]},
				distroID{reRegisteredDistro, guids[reRegisteredDistro]})

			db, err := database.New(dbDir, nil)
			require.NoError(t, err, "Setup: New() should return no error")

			if tc.distroName == reRegisteredDistro {
				guids[reRegisteredDistro] = testutils.ReregisterDistro(t, reRegisteredDistro, false)
			}

			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)
			if tc.breakDBbDump {
				err := os.RemoveAll(dbFile)
				require.NoError(t, err, "Setup: could not remove database dump")
				err = os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: could not create directory to interfere with database dump")
			}
			initialDumpModTime := fileModTime(t, dbFile)

			d, err := db.GetDistroAndUpdateProperties(context.Background(), tc.distroName, tc.props)
			if tc.wantErr {
				require.Error(t, err, "GetDistroAndUpdateProperties should return an error and has not")
				if tc.wantErrType == nil {
					return
				}

				require.ErrorIs(t, err, tc.wantErrType, "GetDistroAndUpdateProperties should return an error of type %T", tc.wantErrType)
				return
			}
			require.NoError(t, err, "GetDistroAndUpdateProperties should return no error when the requested distro is registered")

			require.NotNil(t, d, "GetDistroAndUpdateProperties should return a non-nil distro when the requested one is registered")

			require.Equal(t, tc.distroName, d.Name(), "GetDistroAndUpdateProperties should return a distro with the same name as requested")
			require.Equal(t, guids[tc.distroName], d.GUID(), "GetDistroAndUpdateProperties should return a GUID that matches the requested distro's")
			require.Equal(t, tc.props, d.Properties, "GetDistroAndUpdateProperties should return the same properties as requested")

			// Ensure writing one distro does not modify another
			if tc.distroName != distroInDB {
				d, ok := db.Get(distroInDB)
				require.True(t, ok, "GetDistroAndUpdateProperties should not remove other distros from the database")
				require.NotNil(t, d, "GetDistroAndUpdateProperties should return a non-nil distro when the returned error is nil")

				require.Equal(t, distroInDB, d.Name(), "GetDistroAndUpdateProperties should not modify other distros' name")
				require.Equal(t, guids[distroInDB], d.GUID(), "GetDistroAndUpdateProperties should not modify other distros' GUID")
				require.Equal(t, props[distroInDB], d.Properties, "GetDistroAndUpdateProperties should not modify other distros' properties")
			}

			lastDumpModTime := fileModTime(t, dbFile)
			if tc.wantDbDumpRefreshed {
				require.True(t, lastDumpModTime.After(initialDumpModTime), "GetDistroAndUpdateProperties should modify the database dump file after writing on the database")
				return
			}
			require.Equal(t, initialDumpModTime, lastDumpModTime, "GetDistroAndUpdateProperties should not modify database dump file")
		})
	}
}

func TestDatabaseCleanup(t *testing.T) {
	distro1, guid1 := testutils.RegisterDistro(t, false)
	distro2, guid2 := testutils.RegisterDistro(t, false)

	testCases := map[string]struct {
		reregisterDistro      bool
		markDistroUnreachable string
		breakDbDump           bool

		wantDistros       []string
		wantDumpRefreshed bool
	}{
		"Success with no changes":    {wantDistros: []string{distro1, distro2}},
		"Remove unregistered distro": {reregisterDistro: true, wantDumpRefreshed: true, wantDistros: []string{distro1, distro2}},
		"Remove unreachable distro":  {markDistroUnreachable: distro2, wantDumpRefreshed: true, wantDistros: []string{distro1}},

		"Error on unwritable db file after removing an unregistered distro": {markDistroUnreachable: distro2, breakDbDump: true, wantDumpRefreshed: false, wantDistros: []string{distro1}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			dbDir := t.TempDir()
			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)

			distros := []distroID{
				{distro1, guid1},
				{distro2, guid2}}

			var reregisteredDistro string
			if tc.reregisterDistro {
				var guid string
				reregisteredDistro, guid = testutils.RegisterDistro(t, false)
				distros = append(distros, distroID{reregisteredDistro, guid})
			}

			databaseFromTemplate(t, dbDir, distros...)

			db, err := database.New(dbDir, nil)
			require.NoError(t, err, "Setup: New() should have returned no error")

			if tc.markDistroUnreachable != "" {
				d3, ok := db.Get(distro2)
				require.True(t, ok, "Setup: Distro %q should have been in the database", distro2)
				d3.Invalidate(errors.New("This error should cause the distro to be cleaned up"))
			}

			if tc.reregisterDistro {
				testutils.ReregisterDistro(t, reregisteredDistro, false)
			}

			if tc.breakDbDump {
				err := os.RemoveAll(dbFile)
				require.NoError(t, err, "Setup: when attempting to interfere with a Dump(): could not remove database file")
				err = os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: when attempting to interfere with a Dump(): could not create directory in database file's location")
			}

			initialModTime := fileModTime(t, dbFile)
			fileUpdated := func() bool {
				return initialModTime != fileModTime(t, dbFile)
			}

			db.TriggerCleanup()

			const delay = 500 * time.Millisecond
			if tc.wantDumpRefreshed {
				require.Eventually(t, fileUpdated, delay, 10*time.Millisecond, "Database file should be created after a cleanup when a distro has been unregistered")
			} else {
				time.Sleep(delay)
				require.False(t, fileUpdated(), "Database file should not be refreshed by a cleanup when no distro has been cleaned up")
			}

			require.ElementsMatch(t, tc.wantDistros, db.DistroNames(), "Database contents after cleanup do not match expectations")
		})
	}
}

// fileModTime returns the ModTime of the provided path. If the path
// does not exist, the time is reported as Unix 0.
func fileModTime(t *testing.T, path string) time.Time {
	t.Helper()

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Unix(0, 0)
	}

	require.NoError(t, err, "Could not Stat file %q", path)
	return info.ModTime()
}

// distroID is a convenience struct to package a distro's identifying data.
// Used to deanonymize fixtures.
type distroID struct {
	Name string
	GUID string
}

// databaseFromTemplate creates a yaml database file in the specified directory.
// The template must be in {TestFamilyPath}/database_template.yaml and it'll be
// instantiated in {dest}/database.yaml.
func databaseFromTemplate(t *testing.T, dest string, distros ...distroID) {
	t.Helper()

	in, err := os.ReadFile(filepath.Join(testutils.TestFamilyPath(t), "database_template.yaml"))
	require.NoError(t, err, "Setup: could not read database template")

	tmpl := template.Must(template.New(t.Name()).Parse(string(in)))

	f, err := os.Create(filepath.Join(dest, consts.DatabaseFileName))
	require.NoError(t, err, "Setup: could not create database file")

	err = tmpl.Execute(f, distros)
	require.NoError(t, err, "Setup: could not execute template database file")

	f.Close()
}

// structuredDump is a convenience struct used to parse the database dump and make
// assertions on it with better accuracy that just a strings.Contains.
type structuredDump struct {
	data []database.SerializableDistro
}

// newStructuredDump takes a database dump and parses it to generate a structuredDump.
func newStructuredDump(t *testing.T, rawDump []byte) structuredDump {
	t.Helper()

	var data []database.SerializableDistro

	err := yaml.Unmarshal(rawDump, &data)
	require.NoError(t, err, "In an attempt to parse a database dump: Unmarshal failed for dump:\n%s", rawDump)

	return structuredDump{data: data}
}

// anonymise takes a structured dump and removes all dynamically-generated information,
// leaving behind only information that is invariant across test runs.
func (sd *structuredDump) anonymise(t *testing.T) {
	t.Helper()

	for i := range sd.data {
		sd.data[i].Name = fmt.Sprintf("%%DISTRONAME%d%%", i)
		sd.data[i].GUID = fmt.Sprintf("%%GUID%d%%", i)
	}
}
