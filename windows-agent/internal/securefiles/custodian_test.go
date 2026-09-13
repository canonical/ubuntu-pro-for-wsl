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
	"github.com/sirupsen/logrus"
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
			c, err := securefiles.Open(dir)
			require.NoError(t, err)
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

		"Purge keeps nodes it cannot remove": {
			op:            "purge",
			seedFiles:     map[string]string{"junk.txt": "junk"},
			readOnlyRoot:  true,
			noErr:         true,
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
				removed, opErr = c.Purge(func(string) bool { return false })
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

func TestSetMockOwned(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	require.NoError(t, c.WriteFile("own.txt", []byte("x")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "foreign.txt"), []byte("x"), 0600))

	// The mock overrides the platform predicate in both directions.
	ownedTrue := true
	c.SetMockOwned(&ownedTrue)
	owned, err := c.IsOwned("foreign.txt")
	require.NoError(t, err)
	require.True(t, owned, "Mocked ownership should report a foreign node as owned")

	ownedFalse := false
	c.SetMockOwned(&ownedFalse)
	owned, err = c.IsOwned("own.txt")
	require.NoError(t, err)
	require.False(t, owned, "Mocked ownership should report an owned node as foreign")

	// A nil mock restores the platform behaviour. Linux reports a raw node as
	// cleanly unowned (the xattr is simply missing); on Windows a completely
	// EA-less file has no clean answer, so the predicate errors instead; the
	// attribute-less fallback platforms recognise every node.
	c.SetMockOwned(nil)
	owned, err = c.IsOwned("foreign.txt")
	switch runtime.GOOS {
	case "windows":
		require.Error(t, err, "IsOwned on an EA-less file errors on Windows")
		require.False(t, owned)
	case "linux":
		require.NoError(t, err)
		require.False(t, owned)
	default:
		require.NoError(t, err)
		require.True(t, owned)
	}
}

func TestDegradedModeOperationsAndLogging(t *testing.T) {
	dir := t.TempDir()

	hook := test.NewGlobal()
	defer hook.Reset()

	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer c.Close()

	c.SetMockDegraded(true)
	require.True(t, c.IsDegraded())

	// Test creation and requests serve in degraded mode
	require.NoError(t, os.Mkdir(filepath.Join(dir, "degraded_dir"), 0o700))

	err = c.WriteFile("degraded_dir/degraded_file.txt", []byte("degraded content"))
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "degraded_dir", "degraded_file.txt"))
	require.NoError(t, err)
	require.Equal(t, "degraded content", string(content))

	// In degraded mode the ownership predicate falls back to recognising every node.
	owned, err := c.IsOwned("degraded_dir/degraded_file.txt")
	require.NoError(t, err)
	require.True(t, owned, "Degraded custodian should recognise every node as owned")

	// Verify logging on Open when degraded
	c2, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer c2.Close()
	c2.SetMockDegraded(true)

	// Simulate log on startup/init
	c2.LogDegradedOnce()

	foundError := false
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.ErrorLevel {
			foundError = true
			break
		}
	}
	require.True(t, foundError, "Expected error level log message in degraded mode")
}
