package testutils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

var update bool

const (
	// UpdateGoldenFilesEnv is the environment variable used to indicate go test that
	// the golden files should be overwritten with the current test results.
	UpdateGoldenFilesEnv = `TESTS_UPDATE_GOLDEN`
)

func init() {
	if os.Getenv(UpdateGoldenFilesEnv) != "" {
		update = true
	}
}

type goldenOptions struct {
	goldenPath string
}

// Option is a supported option reference to change the golden files comparison.
type Option func(*goldenOptions)

// WithGoldenPath overrides the default path for golden files used.
func WithGoldenPath(path string) Option {
	return func(o *goldenOptions) {
		if path != "" {
			o.goldenPath = path
		}
	}
}

// LoadWithUpdateFromGolden loads the element from a plaintext golden file.
// It will update the file if the update flag is used prior to loading it.
func LoadWithUpdateFromGolden(t *testing.T, data string, opts ...Option) string {
	t.Helper()

	o := goldenOptions{
		goldenPath: Path(t),
	}

	for _, opt := range opts {
		opt(&o)
	}

	if update {
		t.Logf("updating golden file %s", o.goldenPath)
		err := os.MkdirAll(filepath.Dir(o.goldenPath), 0750)
		require.NoError(t, err, "Cannot create directory for updating golden files")
		err = os.WriteFile(o.goldenPath, []byte(data), 0600)
		require.NoError(t, err, "Cannot write golden file")
	}

	want, err := os.ReadFile(o.goldenPath)
	t.Log(o.goldenPath)
	require.NoError(t, err, "Cannot load golden file")

	r := string(want)
	if runtime.GOOS == "windows" {
		r = strings.ReplaceAll(r, "\r\n", "\n")
	}
	return r
}

// LoadWithUpdateFromGoldenYAML load the generic element from a YAML serialized golden file.
// It will update the file if the update flag is used prior to deserializing it.
func LoadWithUpdateFromGoldenYAML[E any](t *testing.T, got E, opts ...Option) E {
	t.Helper()

	t.Logf("Serializing object for golden file")
	data, err := yaml.Marshal(got)
	require.NoError(t, err, "Cannot serialize provided object")
	want := LoadWithUpdateFromGolden(t, string(data), opts...)

	var wantDeserialized E
	err = yaml.Unmarshal([]byte(want), &wantDeserialized)
	require.NoError(t, err, "Cannot create expanded policy objects from golden file")

	return wantDeserialized
}

// TestFamilyPath returns the path of the dir for storing fixtures and other files related to the test.
func TestFamilyPath(t *testing.T) string {
	t.Helper()

	// Ensures that only the name of the parent test is used.
	familyName, _, _ := strings.Cut(t.Name(), "/")

	return filepath.Join("testdata", familyName)
}

// TestFixturePath returns the path of the dir or file for storing fixture specific to the subtest name.
func TestFixturePath(t *testing.T) string {
	t.Helper()

	// Ensures that only the name of the parent test is used.
	familyName, subtestName, _ := strings.Cut(t.Name(), "/")

	return filepath.Join("testdata", familyName, normalizeName(t, subtestName))
}

// Path returns the golden path for the provided test.
func Path(t *testing.T) string {
	t.Helper()

	path := filepath.Join(TestFamilyPath(t), "golden")
	_, sub, found := strings.Cut(t.Name(), "/")
	if found {
		path = filepath.Join(path, normalizeName(t, sub))
	}

	return path
}

// normalizeName returns a path from name with illegal Windows
// characters replaced or removed.
func normalizeName(t *testing.T, name string) string {
	t.Helper()

	name = strings.ReplaceAll(name, `\`, "_")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ToLower(name)
	return name
}

// UpdateEnabled is a getter for the update flag.
func UpdateEnabled() bool {
	return update
}
