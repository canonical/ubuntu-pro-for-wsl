// Cross-platform mutation semantics: atomic writes leave no temporaries and
// tolerate leftover ones, and Remove/Rename/RemoveAll obey containment. The
// Windows-specific sharing-semantics race lives in mutation_windows_test.go.

package securefiles_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
)

func TestWriteFile(t *testing.T) {
	t.Parallel()

	const leftoverTemp = ".tmp-target.txt-12345678"

	testCases := map[string]struct {
		plantLeftoverTemp bool
		writes            int
	}{
		"a write publishes its content":                 {writes: 1},
		"repeated writes publish the last one":          {writes: 50},
		"a write succeeds despite a leftover temporary": {plantLeftoverTemp: true, writes: 1},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			var wantTemps []string
			if tc.plantLeftoverTemp {
				require.NoError(t, os.WriteFile(filepath.Join(dir, leftoverTemp), []byte("leftover"), 0600),
					"Setup: could not plant the leftover temporary")
				wantTemps = append(wantTemps, leftoverTemp)
			}

			var want string
			for i := range tc.writes {
				want = fmt.Sprintf("write %d", i)
				require.NoError(t, c.WriteFile("target.txt", []byte(want)), "every write must succeed")
			}

			got, err := os.ReadFile(filepath.Join(dir, "target.txt"))
			require.NoError(t, err, "the published node must be readable")
			require.Equal(t, want, string(got), "the published node must hold the last write")

			entries, err := os.ReadDir(dir)
			require.NoError(t, err, "the sub-tree must be listable")

			var gotTemps []string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".tmp-") {
					gotTemps = append(gotTemps, entry.Name())
				}
			}
			// A write cleans up after itself, but it is not a sweep: a temporary left by
			// something else stays until Purge, so only what was planted may remain.
			require.ElementsMatch(t, wantTemps, gotTemps, "a write must not leave its own temporary behind")
		})
	}
}

// TestRename covers publishing by rename. The source must already carry the ownership
// stamp: a node created outside the custodian may still be held open by whoever made it,
// and stamping it after publication would hand that descriptor a root-owned node, which
// ADR 2.01 forbids.
func TestRename(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		from string
		to   string
		// noSource leaves the source out: either it is deliberately absent, or its path
		// lies outside the sub-tree and so cannot be created through the custodian at all.
		noSource bool
		// plantUnowned writes the source behind the custodian's back, so it carries no stamp.
		plantUnowned bool

		wantErr   bool
		wantErrIs error
	}{
		"an owned node is published":                    {from: "source.txt", to: "published.txt"},
		"a node the custodian does not own is refused":  {from: "source.txt", to: "published.txt", plantUnowned: true, wantErr: true, wantErrIs: securefiles.ErrNotOwned},
		"a source that is not there is reported":        {from: "source.txt", to: "published.txt", noSource: true, wantErr: true},
		"a source outside the sub-tree is refused":      {from: "../escape.txt", to: "published.txt", noSource: true, wantErr: true, wantErrIs: securefiles.ErrPathEscapes},
		"a destination outside the sub-tree is refused": {from: "source.txt", to: "../escape.txt", wantErr: true, wantErrIs: securefiles.ErrPathEscapes},
		"a backslash escape is refused":                 {from: "source.txt", to: "..\\escape.txt", wantErr: true, wantErrIs: securefiles.ErrPathEscapes},
		"an absolute destination is refused":            {from: "source.txt", to: "/etc/passwd", wantErr: true, wantErrIs: securefiles.ErrPathEscapes},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			if tc.plantUnowned && c.IsDegraded() {
				t.Skip("a filesystem that cannot carry the stamp has no ownership to verify")
			}

			if !tc.noSource {
				if tc.plantUnowned {
					require.NoError(t, os.WriteFile(filepath.Join(dir, tc.from), []byte("payload"), 0600),
						"Setup: could not plant the unstamped node")
				} else {
					require.NoError(t, c.WriteFile(tc.from, []byte("payload")), "Setup: could not seed the source")
				}
			}

			err = c.Rename(tc.from, tc.to)

			if !tc.wantErr {
				require.NoError(t, err, "the rename should have succeeded")
				require.NoFileExists(t, filepath.Join(dir, tc.from), "the source must be gone once published")

				got, err := os.ReadFile(filepath.Join(dir, tc.to))
				require.NoError(t, err, "the published node must be readable")
				require.Equal(t, "payload", string(got), "the rename must carry the content over")
				return
			}

			if tc.wantErrIs != nil {
				require.ErrorIs(t, err, tc.wantErrIs, "unexpected error kind")
			} else {
				require.Error(t, err, "the rename should have been reported as failed")
			}

			require.NoFileExists(t, filepath.Join(dir, tc.to), "nothing may be published by a failed rename")
			if !tc.noSource {
				require.FileExists(t, filepath.Join(dir, tc.from), "the refused source must stay where it was")
			}
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path string
		// recursive selects RemoveAll over Remove.
		recursive bool
		// seedFile and seedTree say what to create first; an escaping path gets neither,
		// because the custodian refuses to create it just as it refuses to remove it.
		seedFile bool
		seedTree bool

		wantErrIs error
	}{
		"a file is removed":                           {path: "victim.txt", seedFile: true},
		"a populated sub-tree is removed recursively": {path: "tree", recursive: true, seedTree: true},
		"a path outside the sub-tree is refused":      {path: "../escape.txt", wantErrIs: securefiles.ErrPathEscapes},
		"a backslash escape is refused":               {path: "..\\escape.txt", wantErrIs: securefiles.ErrPathEscapes},
		"an absolute path is refused":                 {path: "/etc/passwd", wantErrIs: securefiles.ErrPathEscapes},
		"a recursive escape is refused":               {path: "../escape.txt", recursive: true, wantErrIs: securefiles.ErrPathEscapes},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			switch {
			case tc.seedTree:
				sub, err := c.Subdir(tc.path)
				require.NoError(t, err, "Setup: could not create the sub-tree")
				require.NoError(t, sub.WriteFile("nested.txt", []byte("nested")), "Setup: could not fill the sub-tree")
				// The nested custodian holds an open handle on the sub-tree: on Windows it
				// cannot be removed until that handle is closed, so close it here to keep
				// the test portable.
				require.NoError(t, sub.Close(), "Setup: could not close the nested custodian")
			case tc.seedFile:
				require.NoError(t, c.WriteFile(tc.path, []byte("payload")), "Setup: could not seed the node")
			}

			if tc.recursive {
				err = c.RemoveAll(tc.path)
			} else {
				err = c.Remove(tc.path)
			}

			if tc.wantErrIs != nil {
				require.ErrorIs(t, err, tc.wantErrIs, "unexpected error kind")
				return
			}

			require.NoError(t, err, "the removal should have succeeded")
			require.NoFileExists(t, filepath.Join(dir, tc.path), "the node must be gone")
		})
	}
}
