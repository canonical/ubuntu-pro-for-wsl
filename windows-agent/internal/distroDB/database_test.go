package distroDB_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distroDB"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows"
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

			db, err := distroDB.New(dbDir)
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

	db, err := distroDB.New(databaseDir)
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

			require.Equal(t, d.Name, tc.distroName, "The distro returned by Get should match the one in the database")
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

			db, err := distroDB.New(dbDir)
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

				idx1 := slices.IndexFunc(sd.data, func(s distroDB.SerializableDistro) bool { return s.Name == distro1 })
				idx2 := slices.IndexFunc(sd.data, func(s distroDB.SerializableDistro) bool { return s.Name == distro2 })

				require.NotEqualf(t, -1, idx1, "Database dump should contain distro1 (%s). Dump:\n%s", distro1, dump)
				require.NotEqualf(t, -1, idx2, "Database dump should contain distro2 (%s). Dump:\n%s", distro2, dump)

				require.Equal(t, sd.data[idx1].GUID, guid1.String(), "Database dump GUID for distro1 should match the one it was constructed with. Dump:\n%s", dump)
				require.Equal(t, sd.data[idx2].GUID, guid2.String(), "Database dump GUID for distro2 should match the one it was constructed with. Dump:\n%s", dump)
			}

			// Anonymizing
			sd.anonymise(t)

			// Testing against and optionally updating golden file
			want := testutils.LoadWithUpdateFromGoldenYAML(t, sd.data)
			require.Equal(t, want, sd.data, "Database dump should match expected format")
		})
	}
}

func fileModTime(t *testing.T, path string) time.Time {
	t.Helper()

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Unix(0, 0)
	}

	require.NoError(t, err, "Could not Stat file %q", path)
	return info.ModTime()
}

type distroID struct {
	Name string
	GUID windows.GUID
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

type structuredDump struct {
	data []distroDB.SerializableDistro
}

func newStructuredDump(t *testing.T, rawDump []byte) structuredDump {
	t.Helper()

	var data []distroDB.SerializableDistro

	err := yaml.Unmarshal(rawDump, &data)
	require.NoError(t, err, "In attempt to parse a database dump: Unmarshal failed for dump:\n%s", rawDump)

	return structuredDump{data: data}
}

func (sd *structuredDump) anonymise(t *testing.T) {
	t.Helper()

	for i := range sd.data {
		sd.data[i].Name = fmt.Sprintf("%%DISTRONAME%d%%", i)
		sd.data[i].GUID = fmt.Sprintf("%%GUID%d%%", i)
	}
}
