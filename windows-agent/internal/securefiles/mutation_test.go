// Cross-platform mutation semantics: atomic writes leave no temporaries and
// tolerate leftover ones, and Remove/Rename/RemoveAll obey containment. The
// Windows-specific sharing-semantics race lives in mutation_windows_test.go.

package securefiles_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// TestCreateFileModes covers the choice the agent's log depends on: the rotation that
// precedes it can fail, and the file it would have moved away is then the only copy of
// the record needed to explain why. Appending keeps it; replacing does not.
func TestCreateFileModes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// seedStamped writes the file through the custodian first; seedRaw plants it
		// behind the custodian's back, so it carries no watermark.
		seedStamped string
		seedRaw     string
		// append selects the mode under test. Omitted, CreateFile replaces.
		append bool
		// name overrides the node to create, to reach outside the sub-tree.
		name string

		wantErr     error
		wantContent string
		// wantOwned is only asserted when checkOwned is set: an unstamped node has no
		// portable answer, since Windows fails the attribute query outright while Linux
		// reports a missing one. TestIsOwned pins each shape per platform.
		checkOwned bool
		wantOwned  bool
	}{
		"replacing an absent file creates and stamps it": {
			wantContent: "written",
			checkOwned:  true,
			wantOwned:   true,
		},
		"replacing an existing file discards its content": {
			seedStamped: "earlier",
			wantContent: "written",
			checkOwned:  true,
			wantOwned:   true,
		},
		"appending to an absent file creates and stamps it": {
			append:      true,
			wantContent: "written",
			checkOwned:  true,
			wantOwned:   true,
		},
		"appending to an existing file keeps its content": {
			seedStamped: "earlier",
			append:      true,
			wantContent: "earlierwritten",
			checkOwned:  true,
			wantOwned:   true,
		},
		// The rotation failed and the old log is still in place, unstamped because a
		// previous agent wrote it. Discarding it here would destroy the evidence.
		"appending to an unstamped file adopts it rather than discarding it": {
			seedRaw:     "from a previous run",
			append:      true,
			wantContent: "from a previous runwritten",
		},
		"a name that leaves the sub-tree is refused": {
			name:    "../escape.txt",
			append:  true,
			wantErr: securefiles.ErrPathEscapes,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			target := "log"
			if tc.name != "" {
				target = tc.name
			}
			if tc.seedStamped != "" {
				require.NoError(t, c.WriteFile(target, []byte(tc.seedStamped)), "Setup: could not seed the stamped file")
			}
			if tc.seedRaw != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte(tc.seedRaw), 0600),
					"Setup: could not plant the unstamped file")
			}

			var modes []securefiles.CreateMode
			if tc.append {
				modes = append(modes, securefiles.Append)
			}

			f, err := c.CreateFile(target, modes...)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "unexpected error creating %s", target)
				return
			}
			require.NoError(t, err, "creating %s must succeed", target)

			_, err = f.Write([]byte("written"))
			require.NoError(t, err, "the write must succeed")
			require.NoError(t, f.Close(), "the file must close cleanly")

			got, err := os.ReadFile(filepath.Join(dir, target))
			require.NoError(t, err, "the node must be readable")
			require.Equal(t, tc.wantContent, string(got), "unexpected content for the chosen mode")

			if tc.checkOwned {
				owned, err := c.IsOwned(target)
				require.NoError(t, err, "ownership of the node must be answerable")
				require.Equal(t, tc.wantOwned, owned, "unexpected ownership")
			}
		})
	}
}

// TestCreateFileRevokesOpenDescriptors pins why replacement unlinks rather than truncates.
// The node under test is one that was visible unstamped, so an unprivileged process in a
// distro may already hold it open. Truncating leaves that process attached to the file
// object and reading whatever is written next; unlinking leaves it holding the old node,
// which is what ADR 2.01 means by replacing rather than repairing.
func TestCreateFileRevokesOpenDescriptors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mode []securefiles.CreateMode
		// viaWriteFile publishes through WriteFile instead of CreateFile.
		viaWriteFile bool

		// wantHeldContent is what a descriptor opened before the call reads afterwards.
		wantHeldContent string
		// refusedOnWindows marks the cases Windows answers by refusing instead: it will
		// not unlink a node another handle holds without delete sharing. Refusing is a
		// safe answer to the same question, because nothing was replaced and so nothing
		// written afterwards can reach the holder.
		refusedOnWindows bool
	}{
		"replacing leaves an open descriptor on the old node": {wantHeldContent: "OLD", refusedOnWindows: true},
		// WriteFile publishes by rename onto the name, which likewise never writes
		// through the old node. This is also what makes a torn read impossible: a reader
		// holding the file sees one whole version or the other, never a blend. Windows
		// refuses it for the same reason as above, which is what the polite and impolite
		// readers of TestRenameOverOpenDestination pin.
		"publishing by rename leaves an open descriptor on the old node": {
			viaWriteFile:     true,
			wantHeldContent:  "OLD",
			refusedOnWindows: true,
		},
		// Appending deliberately keeps the same node, so the descriptor follows it. That
		// is the whole point of the mode, and the reason it is only for the agent's log.
		"appending keeps the descriptor on the same node": {
			mode:            []securefiles.CreateMode{securefiles.Append},
			wantHeldContent: "OLDNEW",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			require.NoError(t, c.WriteFile("target.txt", []byte("OLD")), "Setup: could not seed the node")

			held, err := os.Open(filepath.Join(dir, "target.txt"))
			require.NoError(t, err, "Setup: could not hold the node open")
			defer func() { _ = held.Close() }()

			if tc.viaWriteFile {
				err := c.WriteFile("target.txt", []byte("NEW"))
				if tc.refusedOnWindows && runtime.GOOS == "windows" {
					require.Error(t, err, "Windows must refuse to replace a node held open without delete sharing")
					return
				}
				require.NoError(t, err, "publishing must succeed")
				got, err := io.ReadAll(held)
				require.NoError(t, err, "the held descriptor must still be readable")
				require.Equal(t, tc.wantHeldContent, string(got),
					"a descriptor opened before the call must not read content written after it")
				return
			}

			f, err := c.CreateFile("target.txt", tc.mode...)
			if tc.refusedOnWindows && runtime.GOOS == "windows" {
				require.Error(t, err, "Windows must refuse to unlink a node held open without delete sharing")
				return
			}
			require.NoError(t, err, "creating over a node held open must succeed")
			_, err = f.Write([]byte("NEW"))
			require.NoError(t, err, "the write must succeed")
			require.NoError(t, f.Close(), "the new node must close cleanly")

			got, err := io.ReadAll(held)
			require.NoError(t, err, "the held descriptor must still be readable")
			require.Equal(t, tc.wantHeldContent, string(got),
				"a descriptor opened before the call must not read content written after it")
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
