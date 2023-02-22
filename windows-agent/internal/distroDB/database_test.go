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

func (sd *structuredDump) dump(t *testing.T) []byte {
	out, err := yaml.Marshal(sd.data)
	require.NoError(t, err, "In attempt to anonymise a database dump: Marshal failed for dump:\n%s")
	return out
}
