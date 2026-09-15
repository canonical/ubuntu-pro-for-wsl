// Cross-platform contract tests for the custodian: containment, fresh-start
// creation, modes, error paths and constructor behavior must hold on every
// platform, independent of how ownership is stamped (Windows EAs, the Linux
// xattr watermark, or the attribute-less fallback). The platform mechanisms
// themselves are verified in the tagged files of this package.

package securefiles_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func TestCustodian(t *testing.T) {
	t.Parallel()

	escapePaths := []string{
		"../outside.txt",
		"..\\outside.txt",
		"/etc/passwd",
		"sub/../../outside.txt",
	}
	if runtime.GOOS == "windows" {
		escapePaths = append(escapePaths,
			"C:\\Windows\\System32",
			"C:relative_drive_path.txt",
			"D:foo/bar",
		)
	}

	testCases := map[string]struct {
		// useSub makes the case run against the nested custodian c.Subdir("sub") instead of c.
		useSub bool

		// neverOpened runs against a zero-value custodian that Open never touched, the
		// state a caller reaches by declaring one and skipping the error path.
		neverOpened bool

		// closeTwice closes the custodian a second time, which must be a safe no-op.
		closeTwice bool

		// mkdirs are created with os.Mkdir before any other action.
		mkdirs []string

		// seedFiles are written directly via os.WriteFile before the custodian acts.
		seedFiles map[string]string

		// writeFiles are written via c.WriteFile before checking escapes/modes.
		writeFiles map[string]string

		// escapePaths must be rejected by both WriteFile and Mkdir with ErrPathEscapes.
		escapePaths []string

		// testSymlinkEscape adds a check that a symlink pointing outside the root is refused.
		testSymlinkEscape bool

		// wantFileMode lists files whose mode must equal securefiles.FileMode (non-Windows).
		wantFileMode []string

		// checkBasePath asserts BasePath is absolute and, with useSub, sub path is correct.
		checkBasePath bool

		// freshOp selects a single fresh-start operation on a pre-seeded file.
		// Valid value: "CreateFile".
		freshOp string

		// freshFile is the target file for freshOp.
		freshFile string

		// freshContent is written through the opened/created file.
		freshContent string

		// wantFileContents lists files whose content is read back through the custodian.
		wantFileContents map[string]string

		// wantDirEntries lists directories whose entries are listed through the custodian.
		wantDirEntries map[string][]string
	}{
		"a custodian closed twice": {closeTwice: true},

		"a custodian that was never opened": {neverOpened: true},

		"path escapes are refused": {
			escapePaths:       escapePaths,
			testSymlinkEscape: true,
		},
		"sub-custodian scopes operations": {
			useSub:      true,
			writeFiles:  map[string]string{"file.txt": "sub content"},
			escapePaths: []string{"../sibling.txt"},
		},
		"custodian sets modes on new nodes": {
			writeFiles:   map[string]string{"myfile.txt": "hello"},
			wantFileMode: []string{"myfile.txt"},
		},
		"BasePath returns absolute path for root and sub custodians": {
			useSub:        true,
			checkBasePath: true,
		},
		"CreateFile replaces an existing node": {
			seedFiles:    map[string]string{"fresh.txt": "STALE"},
			freshOp:      "CreateFile",
			freshFile:    "fresh.txt",
			freshContent: "NEW",
		},
		"reads files and directory entries through the custodian": {
			mkdirs:           []string{"sub"},
			writeFiles:       map[string]string{"a.txt": "A", "sub/b.txt": "B"},
			wantFileContents: map[string]string{"a.txt": "A", "sub/b.txt": "B"},
			wantDirEntries:   map[string][]string{".": {"a.txt", "sub"}, "sub": {"b.txt"}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			var zero securefiles.Custodian
			c := &zero
			if !tc.neverOpened {
				opened, err := securefiles.Open(dir)
				require.NoError(t, err)
				c = opened
			}
			defer func() { _ = c.Close() }()

			target := c
			if tc.useSub {
				sub, err := c.Subdir("sub")
				require.NoError(t, err)
				defer func() { _ = sub.Close() }()
				target = sub
			}

			basePath := target.BasePath()

			for _, name := range tc.mkdirs {
				require.NoError(t, os.Mkdir(filepath.Join(basePath, name), 0o700))
			}
			for name, content := range tc.seedFiles {
				require.NoError(t, os.WriteFile(filepath.Join(basePath, name), []byte(content), 0600))
			}
			for name, content := range tc.writeFiles {
				require.NoError(t, target.WriteFile(name, []byte(content)))
			}

			for _, path := range tc.escapePaths {
				require.ErrorIs(t, target.WriteFile(path, []byte("data")), securefiles.ErrPathEscapes,
					"Expected ErrPathEscapes for WriteFile name: %s", path)
			}

			if tc.testSymlinkEscape {
				outsideDir := t.TempDir()
				symlinkPath := filepath.Join(c.BasePath(), "symlink_dir")
				if err := os.Symlink(outsideDir, symlinkPath); err == nil {
					require.ErrorIs(t, c.WriteFile("symlink_dir/file.txt", []byte("data")), securefiles.ErrPathEscapes)
				}
			}

			if runtime.GOOS != "windows" {
				for _, name := range tc.wantFileMode {
					fi, err := os.Stat(filepath.Join(basePath, name))
					require.NoError(t, err)
					require.Equal(t, securefiles.FileMode, fi.Mode().Perm())
				}
			}

			if tc.checkBasePath {
				absRoot, err := filepath.Abs(c.BasePath())
				require.NoError(t, err)
				require.Equal(t, absRoot, c.BasePath())
				if tc.useSub {
					require.Equal(t, filepath.Join(absRoot, "sub"), target.BasePath())
				}
			}

			for name, want := range tc.wantFileContents {
				got, err := os.ReadFile(filepath.Join(basePath, name))
				require.NoError(t, err)
				require.Equal(t, want, string(got))
			}

			for name, want := range tc.wantDirEntries {
				entries, err := target.ReadDir(name)
				require.NoError(t, err)
				got := make([]string, 0, len(entries))
				for _, entry := range entries {
					got = append(got, entry.Name())
				}
				sort.Strings(got)
				require.Equal(t, want, got)
			}

			if tc.freshOp == "CreateFile" {
				f, err := target.CreateFile(tc.freshFile)
				require.NoError(t, err)
				_, err = f.Write([]byte(tc.freshContent))
				require.NoError(t, err)
				require.NoError(t, f.Close())
				content, err := os.ReadFile(filepath.Join(basePath, tc.freshFile))
				require.NoError(t, err)
				require.Equal(t, tc.freshContent, string(content))
			}

			if tc.neverOpened {
				require.False(t, c.IsDegraded(), "a custodian with no platform must not claim to be degraded")
				require.NoError(t, c.CheckProjection(), "a custodian with no platform has nothing to report")
			}

			if tc.closeTwice {
				require.NoError(t, c.Close(), "the first close should release everything cleanly")
				require.NoError(t, c.Close(), "a second close must be a safe no-op")
			}
		})
	}
}

func TestCustodianErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// op is the custodian operation under test: one of "write", "create",
		// "isowned", "subdir", "readdir" or "purge".
		op   string
		path string

		// seedFiles are written directly via os.WriteFile before the operation.
		seedFiles map[string]string

		// seedNonEmptyDir creates a directory containing one file before the operation.
		seedNonEmptyDir string

		// closeFirst closes the custodian before running the operation.
		closeFirst bool

		// readOnlyRoot makes the root directory read-only before the operation.
		// The case is skipped on Windows (the read-only attribute on directories
		// does not block child removal) and when running as root.
		readOnlyRoot bool

		// onlyStampedPlatforms skips the case on platforms without a watermark,
		// where the fallback predicate recognises every node without error.
		onlyStampedPlatforms bool

		// wantEscape requires the operation to fail with ErrPathEscapes;
		// otherwise any error is accepted unless noErr is set.
		wantEscape bool
		noErr      bool

		// wantSurvivors lists files that must still exist after the operation.
		wantSurvivors []string
	}{
		"CreateFile rejects a path escape": {op: "create", path: "../out.txt", wantEscape: true},
		"IsOwned rejects a path escape":    {op: "isowned", path: "../out.txt", wantEscape: true},
		"ReadDir rejects a path escape":    {op: "readdir", path: "../out", wantEscape: true},

		"WriteFile through a plain file fails": {
			op: "write", path: "file/child",
			seedFiles: map[string]string{"file": "x"},
		},
		"CreateFile over a non-empty directory fails": {
			op: "create", path: "busy",
			seedNonEmptyDir: "busy",
		},
		"WriteFile over a non-empty directory fails": {
			op: "write", path: "busy",
			seedNonEmptyDir: "busy",
		},
		"WriteFile into a missing directory fails":  {op: "write", path: "missing/f.txt"},
		"CreateFile into a missing directory fails": {op: "create", path: "missing/f.txt"},

		"Subdir over a plain file fails": {
			op: "subdir", path: "afile",
			seedFiles: map[string]string{"afile": "x"},
		},
		"Subdir through a plain file fails": {
			op: "subdir", path: "file/child",
			seedFiles: map[string]string{"file": "x"},
		},
		"Subdir into a read-only root fails": {
			op: "subdir", path: "newsub",
			readOnlyRoot: true,
		},
		"Subdir rejects a path escape": {op: "subdir", path: "../out", wantEscape: true},

		"ReadDir on a plain file fails": {
			op: "readdir", path: "afile",
			seedFiles: map[string]string{"afile": "x"},
		},
		"IsOwned on a missing node fails": {
			op: "isowned", path: "missing.txt",
			onlyStampedPlatforms: true,
		},

		"Purge on a closed custodian fails":   {op: "purge", closeFirst: true},
		"ReadDir on a closed custodian fails": {op: "readdir", path: "x", closeFirst: true},

		// A node the policy rejected but that could not be removed is the one outcome a
		// purge must not report as success: the caller would carry on believing the
		// sub-tree holds nothing foreign, while the node stays readable by every
		// instance. The failure is surfaced and the node is left where it is.
		"Purge fails when a rejected node cannot be removed": {
			op:            "purge",
			seedFiles:     map[string]string{"junk.txt": "junk"},
			readOnlyRoot:  true,
			wantSurvivors: []string{"junk.txt"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.readOnlyRoot && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directory semantics require a non-root Unix user")
			}
			if tc.onlyStampedPlatforms && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
				t.Skip("platforms without a watermark recognise every node")
			}

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err)
			defer func() { _ = c.Close() }()

			for name, content := range tc.seedFiles {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
			}
			if tc.seedNonEmptyDir != "" {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, tc.seedNonEmptyDir), 0750))
				require.NoError(t, os.WriteFile(filepath.Join(dir, tc.seedNonEmptyDir, "child"), []byte("x"), 0600))
			}
			if tc.readOnlyRoot {
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(dir, 0500))
				//nolint:gosec // G302 - test teardown restores directory permissions.
				t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			}
			if tc.closeFirst {
				require.NoError(t, c.Close())
			}

			var opErr error
			var removed []string
			switch tc.op {
			case "write":
				opErr = c.WriteFile(tc.path, []byte("data"))
			case "create":
				f, err := c.CreateFile(tc.path)
				if err == nil {
					_ = f.Close()
				}
				opErr = err
			case "isowned":
				_, opErr = c.IsOwned(tc.path)
			case "subdir":
				sub, err := c.Subdir(tc.path)
				if err == nil {
					_ = sub.Close()
				}
				opErr = err
			case "readdir":
				_, opErr = c.ReadDir(tc.path)
			case "purge":
				removed, opErr = c.Purge(func(string, bool) bool { return false })
			default:
				t.Fatalf("unknown op %q", tc.op)
			}

			switch {
			case tc.noErr:
				require.NoError(t, opErr)
				require.Empty(t, removed)
			case tc.wantEscape:
				require.ErrorIs(t, opErr, securefiles.ErrPathEscapes)
			default:
				require.Error(t, opErr)
				require.Empty(t, removed, "a failed operation must not report nodes as removed")
			}

			for _, name := range tc.wantSurvivors {
				require.FileExists(t, filepath.Join(dir, name))
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// parentIsFile makes the base path's parent a plain file.
		parentIsFile bool

		// baseIsFile makes the base path itself a plain file.
		baseIsFile bool
	}{
		"Open fails when the parent path is a file":     {parentIsFile: true},
		"Open fails when the base path is a plain file": {baseIsFile: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			parent := filepath.Join(tmp, "parent")
			base := filepath.Join(parent, "base")

			if tc.parentIsFile {
				require.NoError(t, os.WriteFile(parent, []byte("x"), 0600))
			}
			if tc.baseIsFile {
				require.NoError(t, os.MkdirAll(parent, 0750))
				require.NoError(t, os.WriteFile(base, []byte("stale"), 0600))
			}

			c, err := securefiles.Open(base)
			require.Error(t, err)
			require.Nil(t, c)
		})
	}
}

// TestDegradedCustodianServesAndReports pins ADR 2.02: a filesystem that cannot carry
// the watermark degrades the custodian loudly but never closes it. Nodes keep being
// written and read, and the finding is held for CheckProjection rather than logged,
// because the custodian is opened before the agent has a log to write to. What the
// ownership predicate makes of an unverifiable node is pinned per platform, in
// watermark_windows_test.go and watermark_linux_test.go.
func TestDegradedCustodianServesAndReports(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// degraded marks the filesystem as unable to carry the watermark.
		degraded bool

		wantGaps error
	}{
		"a healthy sub-tree serves and reports nothing": {},
		"a degraded sub-tree serves and reports the gap": {
			degraded: true,
			wantGaps: securefiles.ErrDegraded,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			hook := test.NewGlobal()
			defer hook.Reset()

			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			c.SetDegraded(tc.degraded)
			require.Equal(t, tc.degraded, c.IsDegraded(), "unexpected degraded state")

			// Serving is the claim; what a write publishes is pinned in TestWriteFile.
			require.NoError(t, c.WriteFile("served.txt", []byte("content")), "a degraded sub-tree must still serve")

			require.Empty(t, hook.AllEntries(), "the custodian must not log; it reports through CheckProjection")

			if tc.wantGaps == nil {
				require.NoError(t, c.CheckProjection(), "a healthy sub-tree has nothing to report")
				return
			}
			require.ErrorIs(t, c.CheckProjection(), tc.wantGaps, "a degraded sub-tree must be reported")
		})
	}
}
