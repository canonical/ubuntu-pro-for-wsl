package distroDB_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distroDB"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"gopkg.in/yaml.v3"
)

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
