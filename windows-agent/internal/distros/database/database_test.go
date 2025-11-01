package database_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type dbDirState int

const (
	emptyDbDir dbDirState = iota
	goodDbFile
	badDbFile
	badDbFileContents
)

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestNew(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro, guid := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		dirState dbDirState

		wantDistros []string
		wantErr     bool
	}{
		"Success on no pre-exisiting database file": {dirState: emptyDbDir, wantDistros: []string{}},
		"Success at loading distro from database":   {dirState: goodDbFile, wantDistros: []string{distro}},

		"Error with syntax error in database file":             {dirState: badDbFileContents, wantErr: true},
		"Error due to database file exists but cannot be read": {dirState: badDbFile, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dbDir := t.TempDir()
			switch tc.dirState {
			case badDbFile:
				err := os.MkdirAll(filepath.Join(dbDir, consts.DatabaseFileName), 0600)
				require.NoError(t, err, "Setup: could not create folder where database file is supposed to go")
			case badDbFileContents:
				err := os.WriteFile(filepath.Join(dbDir, consts.DatabaseFileName), []byte("\tThis is not\nvalid yaml"), 0600)
				require.NoError(t, err, "Setup: could not write wrong database file")
			case goodDbFile:
				databaseFromTemplate(t, dbDir, distroID{distro, guid})
			}

			db, err := database.New(ctx, dbDir)
			if err == nil {
				defer db.Close(ctx)
			}

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

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestDatabaseGetAll(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, _ := wsltestutils.RegisterDistro(t, ctx, false)
	distro2, _ := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		distros []string

		want []string
	}{
		"empty database":            {},
		"database with one entry":   {distros: []string{distro1}, want: []string{distro1}},
		"database with two entries": {distros: []string{distro1, distro2}, want: []string{distro1, distro2}},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database creation should not fail")
			defer db.Close(ctx)

			for i := range tc.distros {
				_, err := db.GetDistroAndUpdateProperties(ctx, tc.distros[i], distro.Properties{})
				require.NoError(t, err, "Setup: could not add %q to database", tc.distros[i])
			}

			distros := db.GetAll()
			var got []string
			for _, d := range distros {
				got = append(got, d.Name())
			}

			require.ElementsMatch(t, tc.want, got, "Unexpected set of distros returned by GetAll")

			// Testing use after close
			db.Close(ctx)
			require.Panics(t, func() { db.GetAll() }, "Database GetAll should panic when used after Close.")
		})
	}
}

func TestDatabaseGetUnmanaged(t *testing.T) {
	ctx := context.Background()
	uncRoot := t.TempDir()

	if wsl.MockAvailable() {
		t.Parallel()
		ctx = database.WithUNCRootPath(wsl.WithMock(ctx, wslmock.New()), uncRoot)
	}

	// Registers some Ubuntu instances
	var distros []string
	for range 3 {
		distros = append(distros, func() string {
			d, _ := wsltestutils.RegisterDistro(t, ctx, false)
			d = strings.ToLower(d)
			testutils.WriteOsRelease(t, uncRoot, d, "ubuntu-os-release")
			return d
		}())
	}

	// Register one non-Ubuntu instance.
	nonUbuntu, _ := wsltestutils.RegisterDistro(t, ctx, false)
	nonUbuntu = strings.ToLower(nonUbuntu)
	testutils.WriteOsRelease(t, uncRoot, nonUbuntu, "other-os-release")

	testCases := map[string]struct {
		dbDistros []string

		want []string
	}{
		"empty database":            {want: distros},
		"database with one entry":   {dbDistros: []string{distros[0]}, want: distros[1:]},
		"database with two entries": {dbDistros: distros[0:2], want: []string{distros[2]}},
		"no unmanaged distros":      {dbDistros: distros, want: []string{}},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if wsl.MockAvailable() {
				t.Parallel()
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: database creation should not fail")
			defer db.Close(ctx)

			for i := range tc.dbDistros {
				_, err := db.GetDistroAndUpdateProperties(ctx, tc.dbDistros[i], distro.Properties{})
				require.NoError(t, err, "Setup: could not add %q to database", tc.dbDistros[i])
			}

			var gotUnmanaged []string
			for _, d := range db.GetUnmanagedDistros() {
				gotUnmanaged = append(gotUnmanaged, d.Name)
			}

			require.ElementsMatch(t, tc.want, gotUnmanaged, "GetUnmanagedDistros returned unexpected set of distros")
			require.NotElementsMatch(t, tc.dbDistros, gotUnmanaged, "GetUnmanagedDistros should not return a distro in the database")
			require.NotContains(t, gotUnmanaged, nonUbuntu, "GetUnmanagedDistros should not return a non-Ubuntu distro")
		})
	}
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestDatabaseGet(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	registeredDistroInDB, registeredGUID := wsltestutils.RegisterDistro(t, ctx, false)
	registeredDistroNotInDB, _ := wsltestutils.RegisterDistro(t, ctx, false)

	nonRegisteredDistroNotInDB, _ := wsltestutils.NonRegisteredDistro(t)
	nonRegisteredDistroInDB, oldGUID := wsltestutils.RegisterDistro(t, ctx, false)

	databaseDir := t.TempDir()
	databaseFromTemplate(t, databaseDir,
		distroID{registeredDistroInDB, registeredGUID},
		distroID{nonRegisteredDistroInDB, oldGUID})

	db, err := database.New(ctx, databaseDir)
	require.NoError(t, err, "Setup: New() should return no error")

	// Must use Cleanup. If we use defer, it'll run before the subtests are launched.
	t.Cleanup(func() { db.Close(ctx) })

	// Unregister the distro now, so that it's in the db object but not on system properly.
	wsltestutils.UnregisterDistro(t, ctx, nonRegisteredDistroInDB)

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

func TestDatabaseGetAfterClose(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	db, err := database.New(ctx, t.TempDir())
	require.NoError(t, err, "Setup: New() should return no error")

	db.Close(ctx)

	require.Panics(t, func() { db.Get(wsltestutils.RandomDistroName(t)) }, "Database Get should panic when used after Close.")
}

//nolint:tparallel // Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
func TestDatabaseDump(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, guid1 := wsltestutils.RegisterDistro(t, ctx, false)
	distro2, guid2 := wsltestutils.RegisterDistro(t, ctx, false)

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
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dbDir := t.TempDir()

			if !tc.emptyDB {
				databaseFromTemplate(t, dbDir, distroID{distro1, guid1}, distroID{distro2, guid2})
			}

			db, err := database.New(ctx, dbDir)
			require.NoError(t, err, "Setup: empty database should be created without issue")
			defer db.Close(ctx)

			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)
			switch tc.dirState {
			case badDbFile:
				testutils.ReplaceFileWithDir(t, dbFile, "Setup: could not create directory to interfere with database dump")
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
				require.Empty(t, sd.data, "Database dump should contain no distros")
			} else {
				require.Len(t, sd.data, 2, "Database dump should contain exactly two distros")

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

			// Testing use after close
			db.Close(ctx)
			require.Panics(t, func() { _ = db.Dump() }, "Database dump should panic when used after Close.")
		})
	}
}

func TestGetDistroAndUpdateProperties(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	var distroInDB, distroNotInDB, reRegisteredDistro, nonRegisteredDistro string
	var guids map[string]string

	// Scope to avoid leaking guid variables
	{
		var guid1, guid2, guid3, guid4 string

		distroInDB, guid1 = wsltestutils.RegisterDistro(t, ctx, false)
		distroNotInDB, guid2 = wsltestutils.RegisterDistro(t, ctx, false)
		reRegisteredDistro, guid3 = wsltestutils.RegisterDistro(t, ctx, false)
		nonRegisteredDistro, guid4 = wsltestutils.NonRegisteredDistro(t)

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
			Hostname:    "SuperTestMachine",
		},
		distroNotInDB: {
			DistroID:    "HyperUbuntu",
			VersionID:   "222.04",
			PrettyName:  "Ubuntu 122.04 LTS (Joker Jellyfish)",
			ProAttached: false,
			Hostname:    "HyperTestMachine",
		},
		reRegisteredDistro: {
			DistroID:    "Ubuntu",
			VersionID:   "22.04",
			PrettyName:  "Ubuntu 22.04 LTS (Jammy Jellyfish)",
			ProAttached: true,
			Hostname:    "NormalTestMachine",
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

		"Error on distro not in database and we do not add it ": {distroName: nonRegisteredDistro, wantErr: true, wantErrType: &distro.NotValidError{}},
		"Error on database refresh failing":                     {distroName: distroInDB, props: props[distroNotInDB], breakDBbDump: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dbDir := t.TempDir()
			databaseFromTemplate(t, dbDir,
				distroID{distroInDB, guids[distroInDB]},
				distroID{reRegisteredDistro, guids[reRegisteredDistro]})

			db, err := database.New(ctx, dbDir)
			require.NoError(t, err, "Setup: New() should return no error")
			defer db.Close(ctx)

			if tc.distroName == reRegisteredDistro {
				guids[reRegisteredDistro] = wsltestutils.ReregisterDistro(t, ctx, reRegisteredDistro, false)
			}

			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)
			if tc.breakDBbDump {
				testutils.ReplaceFileWithDir(t, dbFile, "Setup: could not create directory to interfere with database dump")
			}
			initialDumpModTime := fileModTime(t, dbFile)
			time.Sleep(100 * time.Millisecond) // Prevents modtime precision issues

			d, err := db.GetDistroAndUpdateProperties(ctx, tc.distroName, tc.props)
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
			require.Equal(t, tc.props, d.Properties(), "GetDistroAndUpdateProperties should return the same properties as requested")

			// Ensure writing one distro does not modify another
			if tc.distroName != distroInDB {
				d, ok := db.Get(distroInDB)
				require.True(t, ok, "GetDistroAndUpdateProperties should not remove other distros from the database")
				require.NotNil(t, d, "GetDistroAndUpdateProperties should return a non-nil distro when the returned error is nil")

				require.Equal(t, distroInDB, d.Name(), "GetDistroAndUpdateProperties should not modify other distros' name")
				require.Equal(t, guids[distroInDB], d.GUID(), "GetDistroAndUpdateProperties should not modify other distros' GUID")
				require.Equal(t, props[distroInDB], d.Properties(), "GetDistroAndUpdateProperties should not modify other distros' properties")
			}

			lastDumpModTime := fileModTime(t, dbFile)
			if tc.wantDbDumpRefreshed {
				require.True(t, lastDumpModTime.After(initialDumpModTime), "GetDistroAndUpdateProperties should modify the database dump file after writing on the database")
				return
			}
			require.Equal(t, initialDumpModTime, lastDumpModTime, "GetDistroAndUpdateProperties should not modify database dump file")

			// Testing use after close
			db.Close(ctx)
			require.Panics(t, func() { _, _ = db.GetDistroAndUpdateProperties(ctx, tc.distroName, tc.props) }, "Database GetDistroAndUpdateProperties should panic when used after Close.")
		})
	}
}

func TestDatabaseCleanup(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, guid1 := wsltestutils.RegisterDistro(t, ctx, false)
	distro2, guid2 := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		reregisterDistro      bool
		markDistroUnreachable string
		breakDbDump           bool
		cleanupFunc           bool

		wantDistros       []string
		wantDumpRefreshed bool
		wantCleanup       bool
	}{
		"Success with no changes":     {wantDistros: []string{distro1, distro2}},
		"Remove unregistered distro":  {reregisterDistro: true, wantDumpRefreshed: true, wantDistros: []string{distro1, distro2}},
		"Remove unreachable distro":   {markDistroUnreachable: distro2, wantDumpRefreshed: true, wantDistros: []string{distro1}},
		"Cleanup using callback func": {cleanupFunc: true, markDistroUnreachable: distro2, wantDistros: []string{distro1}, wantDumpRefreshed: true, wantCleanup: true},

		"Error on unwritable db file after removing an unregistered distro": {markDistroUnreachable: distro2, breakDbDump: true, wantDumpRefreshed: false, wantDistros: []string{distro1}},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dbDir := t.TempDir()
			dbFile := filepath.Join(dbDir, consts.DatabaseFileName)

			distros := []distroID{
				{distro1, guid1},
				{distro2, guid2}}

			var reregisteredDistro string
			if tc.reregisterDistro {
				var guid string
				reregisteredDistro, guid = wsltestutils.RegisterDistro(t, ctx, false)
				distros = append(distros, distroID{reregisteredDistro, guid})
			}

			databaseFromTemplate(t, dbDir, distros...)

			var cleanupCalled atomic.Bool
			var cleanupFunc func(string)
			if tc.cleanupFunc {
				cleanupFunc = func(d string) {
					cleanupCalled.Store(strings.EqualFold(tc.markDistroUnreachable, d))
				}
			}

			db, err := database.New(ctx, dbDir, cleanupFunc)
			require.NoError(t, err, "Setup: New() should have returned no error")
			defer db.Close(ctx)

			if tc.markDistroUnreachable != "" {
				d3, ok := db.Get(distro2)
				require.True(t, ok, "Setup: Distro %q should have been in the database", distro2)
				d3.Invalidate(ctx) // This should cause the distro to be cleaned up
			}

			if tc.reregisterDistro {
				wsltestutils.ReregisterDistro(t, ctx, reregisteredDistro, false)
			}

			if tc.breakDbDump {
				err := os.RemoveAll(dbFile)
				require.NoError(t, err, "Setup: when attempting to interfere with a Dump(): could not remove database file")
				err = os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: when attempting to interfere with a Dump(): could not create directory in database file's location")
			}

			initialModTime := fileModTime(t, dbFile)
			time.Sleep(100 * time.Millisecond) // Prevents modtime precision issues

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

			require.Equal(t, tc.wantCleanup, cleanupCalled.Load(), "Cleanup callback state mismatch")

			require.ElementsMatch(t, tc.wantDistros, db.DistroNames(), "Database contents after cleanup do not match expectations")

			// Testing use after close
			db.Close(ctx)
			require.Panics(t, func() { db.TriggerCleanup() }, "Database TriggerCleanup should panic when used after Close.")
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
	defer f.Close()

	err = tmpl.Execute(f, distros)
	require.NoError(t, err, "Setup: could not execute template database file")
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
