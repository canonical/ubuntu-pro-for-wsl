// Cross-platform contract tests for the custodian: containment, fresh-start creation,
// modes, error paths, purge policy and constructor behavior must hold on every
// platform, independent of how ownership is stamped (Windows EAs or the Linux xattr
// watermark). The platform mechanisms themselves are verified in the tagged files of
// this package.

package securefiles_test

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
)

// errSharingViolation is the errno the kernel answers when an open conflicts with a held
// handle that declines to share deletion - the shape of the root veto, and the expected
// answer for a rename or removal issued behind the custodian's back. It is spelled as
// the raw errno because ERROR_SHARING_VIOLATION only exists in the windows-only
// packages a cross-platform test file cannot import.
const errSharingViolation = syscall.Errno(32)

// TestCustodianClosesCleanlyTwice pins that closing is idempotent: the first
// close releases everything, and a second one is a safe no-op.
func TestCustodianClosesCleanlyTwice(t *testing.T) {
	t.Parallel()

	c, err := securefiles.Open(t.TempDir())
	require.NoError(t, err, "Setup: could not open custodian")
	require.NoError(t, c.Close(), "the first close should release everything cleanly")
	require.NoError(t, c.Close(), "a second close must be a safe no-op")
}

// TestZeroValueCustodianIsInert pins the state a caller reaches by declaring a
// custodian and skipping the error path: one Open never touched. It has no
// platform, so there is nothing to report, and closing it touches nothing.
func TestZeroValueCustodianIsInert(t *testing.T) {
	t.Parallel()

	var zero securefiles.Custodian
	require.NoError(t, zero.CheckProjection(), "a custodian with no platform has nothing to report")
	require.NoError(t, zero.Close(), "closing a custodian that was never opened must be a no-op")
}

// TestNewNodesCarryFileMode pins that nodes the custodian creates carry the
// package's file mode, which is what keeps them unreadable to other local
// users. Windows carries the mode in the stamp instead, so the POSIX bits do
// not apply there.
func TestNewNodesCarryFileMode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows nodes carry the mode in the stamp, not in POSIX permission bits")
	}

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	require.NoError(t, c.WriteFile("myfile.txt", []byte("hello")), "the file must be written")
	fi, err := os.Stat(filepath.Join(dir, "myfile.txt"))
	require.NoError(t, err, "the node must be statable")
	require.Equal(t, securefiles.FileMode, fi.Mode().Perm(), "new nodes must carry the package's file mode")
}

// TestBasePathIsAbsoluteForRootAndSubCustodians pins the path the custodian
// reports: the root custodian reports the absolute base path it was resolved
// to, and a sub custodian reports its own sub-tree.
func TestBasePathIsAbsoluteForRootAndSubCustodians(t *testing.T) {
	t.Parallel()

	dir, err := filepath.Abs(t.TempDir())
	require.NoError(t, err, "Setup: could not resolve the temporary directory")
	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	require.Equal(t, dir, c.BasePath(), "the root custodian reports the absolute base path")

	sub, err := c.Subdir("sub")
	require.NoError(t, err, "Setup: could not open the sub custodian")
	defer func() { _ = sub.Close() }()
	require.Equal(t, filepath.Join(dir, "sub"), sub.BasePath(), "the sub custodian reports its own sub-tree")
}

// TestSubCustodianIsScopedToItsSubTree pins what per-distro data isolation
// rests on: a custodian scoped to one sub-tree can write its own nodes, but a
// name that leaves the sub-tree is refused, and nothing reaches a sibling.
func TestSubCustodianIsScopedToItsSubTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sibling := filepath.Join(dir, "sibling.txt")
	require.NoError(t, os.WriteFile(sibling, []byte("sibling"), 0600), "Setup: could not plant the sibling")

	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	sub, err := c.Subdir("sub")
	require.NoError(t, err, "Setup: could not open the sub custodian")
	defer func() { _ = sub.Close() }()

	require.NoError(t, sub.WriteFile("file.txt", []byte("sub content")), "the sub custodian writes inside its sub-tree")
	require.ErrorIs(t, sub.WriteFile("../sibling.txt", []byte("data")), securefiles.ErrPathEscapes,
		"a name that leaves the sub-tree is refused")

	got, err := os.ReadFile(sibling)
	require.NoError(t, err, "the sibling must still be readable")
	require.Equal(t, "sibling", string(got), "the refused write must not have reached the sibling")
}

// TestReadDirListsTheSubTree pins the listing surface the sweep and the
// projection checks rest on: the custodian lists the nodes of its own sub-tree,
// one level at a time.
func TestReadDirListsTheSubTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0700), "Setup: could not create the sub-tree")
	require.NoError(t, c.WriteFile("a.txt", []byte("A")), "Setup: could not write the file")
	require.NoError(t, c.WriteFile(filepath.Join("sub", "b.txt"), []byte("B")), "Setup: could not write the nested file")

	names := func(entries []os.DirEntry) []string {
		got := make([]string, 0, len(entries))
		for _, entry := range entries {
			got = append(got, entry.Name())
		}
		return got
	}

	entries, err := c.ReadDir(".")
	require.NoError(t, err, "the sub-tree must be listable")
	require.Equal(t, []string{"a.txt", "sub"}, names(entries), "unexpected root entries")

	entries, err = c.ReadDir("sub")
	require.NoError(t, err, "the sub-tree must be listable")
	require.Equal(t, []string{"b.txt"}, names(entries), "unexpected sub-tree entries")
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

		// seedSymlink plants a symlink pointing outside the sub-tree, skipped when the
		// environment cannot create one.
		seedSymlink string

		// closeFirst closes the custodian before running the operation.
		closeFirst bool

		// readOnlyRoot makes the root directory read-only before the operation.
		// The case is skipped on Windows (the read-only attribute on directories
		// does not block child removal) and when running as root.
		readOnlyRoot bool

		// wantEscape requires the operation to fail with ErrPathEscapes;
		// otherwise any error is accepted.
		wantEscape bool

		// wantSurvivors lists files that must still exist after the operation.
		wantSurvivors []string
	}{
		"CreateFile rejects a path escape": {op: "create", path: "../out.txt", wantEscape: true},
		"AppendFile rejects a path escape": {op: "appendfile", path: "../out.txt", wantEscape: true},
		"IsOwned rejects a path escape":    {op: "isowned", path: "../out.txt", wantEscape: true},
		"ReadDir rejects a path escape":    {op: "readdir", path: "../out", wantEscape: true},
		"WriteFile rejects a path escape":  {op: "write", path: "../out.txt", wantEscape: true},
		// Drive-qualified names are refused on every platform: the custodian
		// contract is platform-deterministic, so a name that is anchored on
		// Windows is refused where it is merely an odd relative name too.
		"WriteFile rejects a drive-qualified path": {op: "write", path: `C:\Windows\System32`, wantEscape: true},
		"WriteFile rejects a drive-relative path":  {op: "write", path: "D:foo/bar", wantEscape: true},
		// A link as an intermediate component passes lexical resolution; the
		// refusal is the platform layer's physical containment.
		"WriteFile through a link is refused": {op: "write", path: "link/child", seedSymlink: "link"},

		// A node the adoption path cannot even stat must be refused rather than
		// created over: appending only makes sense on a node that is verifiably there.
		"AppendFile through a plain file fails": {
			op: "appendfile", path: "file/child",
			seedFiles: map[string]string{"file": "x"},
		},
		"AppendFile over a reparse point is refused": {
			op: "appendfile", seedSymlink: "link",
		},

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
		},

		// After a close, every operation fails on the closed root. The refusal
		// surfaces at each operation's own error handling, so the shapes with
		// distinct call sites get a pin each: the custodian's own handle
		// (ReadDir), the platform layer's (WriteFile), and the sweeps' entry
		// (Purge, PurgeAll).
		"ReadDir on a closed custodian fails":   {op: "readdir", path: "x", closeFirst: true},
		"WriteFile on a closed custodian fails": {op: "write", path: "f.txt", closeFirst: true},
		"Purge on a closed custodian fails":     {op: "purge", closeFirst: true},
		"PurgeAll on a closed custodian fails":  {op: "purgeall", closeFirst: true},

		"PurgeAll fails when a node cannot be removed": {
			op:            "purgeall",
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

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err)
			defer func() { _ = c.Close() }()

			for name, content := range tc.seedFiles {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
			}
			if tc.seedSymlink != "" {
				if err := os.Symlink(t.TempDir(), filepath.Join(dir, tc.seedSymlink)); err != nil {
					t.Skip("symlink creation not permitted in this environment")
				}
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
			case "appendfile":
				f, err := c.AppendFile(tc.path)
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
			case "purgeall":
				_, opErr = c.PurgeAll()
			default:
				t.Fatalf("unknown op %q", tc.op)
			}

			switch {
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

// TestSubdirReplacesAPlantedReparsePoint verifies that a link planted where a sub-tree
// root must live does not block the agent forever: the link is removed and the directory
// created in its place, serving the child custodian, while the link target is untouched.
// A link that cannot be removed is the refusal case: the sub-tree is refused rather than
// opened through something unchecked. The tree-root refusal pinned in
// TestEnsureRootRefusesARedirectedRoot stays where it is, the one place where a link
// cannot be told from a deliberate redirection.
func TestSubdirReplacesAPlantedReparsePoint(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// linkTargetOutsideDir makes the planted link point at the outside directory;
		// otherwise it points at the outside sentinel file.
		linkTargetOutsideDir bool

		// linkInTree plants the link pointing at a missing sibling inside the tree.
		linkInTree bool

		// cannotRemove makes the planted link unremovable, so the heal cannot run:
		// the sub-tree must be refused, not opened through something unchecked.
		cannotRemove bool
	}{
		"a link to a directory outside the tree": {linkTargetOutsideDir: true},
		"a link to a file outside the tree":      {},

		// A link inside the tree is the subtle one: os.Root resolves it, so a
		// fresh-looking creation may have gone through the link instead of creating
		// the directory itself.
		"a link to a directory inside the tree": {linkInTree: true},

		"a link that cannot be removed is refused": {cannotRemove: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")
			sentinel := filepath.Join(outside, "precious")
			require.NoError(t, os.WriteFile(sentinel, []byte("PRECIOUS"), 0600), "Setup: could not write the sentinel")

			// Pre-plant before the first start: the public directory and the link exist
			// before the custodian does, the state an unprivileged instance user reaches
			// on a fresh install.
			root := filepath.Join(base, "root")
			require.NoError(t, os.MkdirAll(root, 0700), "Setup: could not create the future tree root")
			switch {
			case tc.linkTargetOutsideDir:
				require.NoError(t, os.Symlink(outside, filepath.Join(root, "sub")), "Setup: could not plant the link")
			case tc.linkInTree:
				require.NoError(t, os.Symlink(filepath.Join(root, "sibling"), filepath.Join(root, "sub")), "Setup: could not plant the link")
			default:
				require.NoError(t, os.Symlink(sentinel, filepath.Join(root, "sub")), "Setup: could not plant the link")
			}

			c, err := securefiles.Open(root)
			require.NoError(t, err, "the tree itself is still adoptable and stampable")
			defer func() { _ = c.Close() }()

			if tc.cannotRemove {
				// Make the link unremovable: on Windows a reparse point held open
				// without delete-sharing refuses its own removal; on the other
				// platforms a non-root Unix user cannot remove a link from a tree
				// root they cannot write.
				switch runtime.GOOS {
				case "windows":
					holdReparsePointOpen(t, filepath.Join(root, "sub"))
				default:
					if os.Geteuid() == 0 {
						t.Skip("read-only directory semantics require a non-root Unix user")
					}
					//nolint:gosec // G302 - test setup removes directory write permission.
					require.NoError(t, os.Chmod(root, 0500), "Setup: could not make the tree root read-only")
					//nolint:gosec // G302 - test teardown restores directory permissions.
					t.Cleanup(func() { _ = os.Chmod(root, 0700) })
				}
			}

			sub, err := c.Subdir("sub")
			if tc.cannotRemove {
				require.Error(t, err, "a link that cannot be removed must refuse the sub-tree")
				require.Nil(t, sub, "a refused sub-tree must not yield a custodian")

				fi, err := os.Lstat(filepath.Join(root, "sub"))
				require.NoError(t, err, "the planted link must still stand at the name")
				require.NotZero(t, fi.Mode()&os.ModeSymlink, "the planted link must not have been replaced")
				data, err := os.ReadFile(sentinel)
				require.NoError(t, err)
				require.Equal(t, "PRECIOUS", string(data), "nothing may be written through the link")
				return
			}
			require.NoError(t, err, "a planted link must not block the sub-tree forever")
			defer func() { _ = sub.Close() }()

			// The link target is untouched, ...
			data, err := os.ReadFile(sentinel)
			require.NoError(t, err)
			require.Equal(t, "PRECIOUS", string(data), "nothing may be written through the link")

			// ... the sub-tree root is a real directory again, ...
			fi, err := os.Lstat(filepath.Join(root, "sub"))
			require.NoError(t, err)
			require.True(t, fi.IsDir(), "the planted link must be replaced by the directory")
			require.Zero(t, fi.Mode()&os.ModeSymlink, "the planted link must be gone")

			// ... and it serves the child custodian with stamped content.
			require.NoError(t, sub.WriteFile("data.txt", []byte("data")))
			owned, err := sub.IsOwned("data.txt")
			require.NoError(t, err)
			require.True(t, owned, "content in the replaced sub-tree must carry the watermark")
		})
	}
}

// TestPurge pins the caller-policy sweep: what the policy rejects is removed
// and reported, the sweep carries past a node it cannot remove while the
// failure is surfaced, a vanished root is a failure rather than a success,
// and a sub-custodian's sweep is scoped to its own sub-tree.
func TestPurge(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// needsNonRootUnix skips the case unless a non-root Unix user runs it:
		// the read-only directory semantics the refusal needs do not exist on
		// Windows, and root ignores them.
		needsNonRootUnix bool
		// needsUnix skips the case unless a Unix host runs it: the custodian
		// holds an open handle on the root, so Windows cannot remove it
		// externally.
		needsUnix bool

		makeSub bool
		preseed func(t *testing.T, c *securefiles.Custodian)
		run     func(t *testing.T, c *securefiles.Custodian) []string
		check   func(t *testing.T, c *securefiles.Custodian, removed []string)
	}{
		// A node that resists removal must not become a shelter for the rest: the sweep
		// carries on, the failure is reported, and only the stuck node is left behind.
		"Purge continues past a node it cannot remove": {
			needsNonRootUnix: true,
			preseed: func(t *testing.T, c *securefiles.Custodian) {
				t.Helper()
				basePath := c.BasePath()

				// An unremovable node: a directory whose read-only child cannot be unlinked.
				keep := filepath.Join(basePath, "stuck", "keep")
				require.NoError(t, os.MkdirAll(keep, 0700), "Setup: could not create the obstructing tree")
				require.NoError(t, os.WriteFile(filepath.Join(keep, "child"), []byte("x"), 0600), "Setup: could not fill it")
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(keep, 0500), "Setup: could not make it read-only")
				//nolint:gosec // G302 - test teardown restores directory permissions.
				t.Cleanup(func() { _ = os.Chmod(keep, 0700) })

				require.NoError(t, os.WriteFile(filepath.Join(basePath, "removable.txt"), []byte("x"), 0600), "Setup: could not seed the removable node")
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				removed, err := c.Purge(func(string, bool) bool { return false })
				require.Error(t, err, "an unremovable node must be reported")
				return removed
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				require.Equal(t, []string{"removable.txt"}, removed, "the sweep must continue past the stuck node")
				require.DirExists(t, filepath.Join(c.BasePath(), "stuck"), "the stuck node stays where it is")
				require.NoFileExists(t, filepath.Join(c.BasePath(), "removable.txt"), "the removable node must still be purged")
			},
		},
		"Purge removes unrecognised nodes and temp files while keeping allowed ones": {
			preseed: func(t *testing.T, c *securefiles.Custodian) {
				t.Helper()
				basePath := c.BasePath()
				require.NoError(t, c.WriteFile("legit.txt", []byte("valid")))
				require.NoError(t, os.WriteFile(filepath.Join(basePath, "malicious.txt"), []byte("bad"), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(basePath, ".tmp-legit.txt-1234"), []byte("tmp"), 0600))
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				allowedPolicy := func(relPath string, _ bool) bool { return relPath == "legit.txt" }
				removed, err := c.Purge(allowedPolicy)
				require.NoError(t, err)
				return removed
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				basePath := c.BasePath()
				require.Contains(t, removed, "malicious.txt")
				require.Contains(t, removed, ".tmp-legit.txt-1234")
				require.NotContains(t, removed, "legit.txt")
				require.FileExists(t, filepath.Join(basePath, "legit.txt"))
				require.NoFileExists(t, filepath.Join(basePath, "malicious.txt"))
				require.NoFileExists(t, filepath.Join(basePath, ".tmp-legit.txt-1234"))
			},
		},
		"Purge fails when the root was removed externally": {
			needsUnix: true,
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				require.NoError(t, os.RemoveAll(c.BasePath()))
				_, err := c.Purge(func(string, bool) bool { return false })
				require.Error(t, err, "purging a vanished root must error rather than report success")
				return nil
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
			},
		},
		// The wholesale twin of the case above: PurgeAll has its own sweep, so
		// its vanished-root branch needs its own pin. On POSIX the open root
		// survives an external removal long enough to answer, so the failure
		// surfaces at the listing rather than at the open; Windows cannot run
		// this, the veto that keeps the root in place being pinned by
		// TestTreeRootCannotBeMovedOrRemovedWhileTheCustodianIsOpen.
		"PurgeAll fails when the root was removed externally": {
			needsUnix: true,
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				require.NoError(t, os.RemoveAll(c.BasePath()))
				_, err := c.PurgeAll()
				require.Error(t, err, "purging a vanished root must error rather than report success")
				return nil
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
			},
		},
		"Purge on a nested custodian is scoped to its sub-tree": {
			makeSub: true,
			preseed: func(t *testing.T, c *securefiles.Custodian) {
				t.Helper()
				require.NoError(t, c.WriteFile("keep.txt", []byte("keep")))
				require.NoError(t, c.WriteFile("remove.txt", []byte("remove")))
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				isAllowed := func(relPath string, _ bool) bool { return relPath == "keep.txt" }
				removed, err := c.Purge(isAllowed)
				require.NoError(t, err)
				return removed
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				basePath := c.BasePath()
				require.Equal(t, []string{"remove.txt"}, removed)
				require.FileExists(t, filepath.Join(basePath, "keep.txt"))
				require.NoFileExists(t, filepath.Join(basePath, "remove.txt"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.needsNonRootUnix && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directory semantics require a non-root Unix user")
			}
			if tc.needsUnix && runtime.GOOS == "windows" {
				t.Skip("the custodian holds an open handle on the root, so Windows cannot remove it externally")
			}

			parent := t.TempDir()

			var c *securefiles.Custodian
			if tc.makeSub {
				root, err := securefiles.Open(parent)
				require.NoError(t, err)
				t.Cleanup(func() { _ = root.Close() })
				c, err = root.Subdir("sub")
				require.NoError(t, err)
			} else {
				var err error
				c, err = securefiles.Open(parent)
				require.NoError(t, err)
			}
			defer func() { _ = c.Close() }()

			if tc.preseed != nil {
				tc.preseed(t, c)
			}

			var removed []string
			if tc.run != nil {
				removed = tc.run(t, c)
			}

			tc.check(t, c, removed)
		})
	}
}

// TestPurgeAll verifies that the wholesale wipe removes every node in the sub-tree,
// whatever it is, and leaves the custodian working from its root.
func TestPurgeAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	require.NoError(t, c.WriteFile("data.txt", []byte("data")))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "subdir", "child"), []byte("x"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tmp-leftover"), []byte("x"), 0600))

	removed, err := c.PurgeAll()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"data.txt", "subdir", ".tmp-leftover"}, removed,
		"the wholesale wipe must name every node it removed")

	entries, err := c.ReadDir(".")
	require.NoError(t, err)
	require.Empty(t, entries, "every node in the sub-tree must be removed")

	// The sub-tree root itself is kept: the custodian still works from it.
	require.NoError(t, c.WriteFile("again.txt", []byte("data")))
	owned, err := c.IsOwned("again.txt")
	require.NoError(t, err)
	require.True(t, owned, "content written after the wipe must carry the watermark")
}

// TestTreeRootCannotBeMovedOrRemovedWhileTheCustodianIsOpen pins the root veto:
// while the custodian is open, the tree root cannot be renamed or removed by
// anything else — on Windows by share arbitration, which binds every issuer
// alike, the 9p server performing an instance's mv included. POSIX cannot
// refuse a rename because a descriptor is open, so there the tree moves
// freely and the runtime root-swap window stays open — its closure is the
// instance-side validation, not the custodian (ADR 2.01).
func TestTreeRootCannotBeMovedOrRemovedWhileTheCustodianIsOpen(t *testing.T) {
	t.Parallel()

	tree := filepath.Join(t.TempDir(), "tree")
	c, err := securefiles.Open(tree)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		err = os.Rename(tree, tree+".moved")
		require.ErrorIs(t, err, errSharingViolation,
			"the held root must veto a rename issued behind the custodian's back, got: %v", err)

		err = os.Remove(tree)
		require.ErrorIs(t, err, errSharingViolation,
			"the held root must veto a removal issued behind the custodian's back, got: %v", err)

		require.NoError(t, c.Close())

		// With the veto released, a plain mover succeeds again.
		require.NoError(t, os.Rename(tree, tree+".moved"))
		require.NoError(t, os.Rename(tree+".moved", tree))
		return
	}

	// Linux: POSIX rename semantics cannot be vetoed by an open descriptor.
	require.NoError(t, os.Rename(tree, tree+".moved"))
	require.NoError(t, os.Rename(tree+".moved", tree))
	require.NoError(t, c.Close())
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

// TestCheckProjection pins what the custodian reports about its own sub-tree, and that a
// healthy one reports nothing. A volume this machine does not own stores the stamp
// faithfully, so a custodian that looks healthy from Windows is not evidence that
// instances see a root-owned sub-tree; ADR 2.02 requires the condition to be named
// rather than refused. The paths are names a test machine does not have: the custodian
// is constructed over them without establishing anything.
func TestCheckProjection(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		basePath string

		// wantErrs lists every condition CheckProjection must name. Empty means a
		// healthy sub-tree, for which it says nothing at all.
		wantErrs []error
		wantText string
	}{
		"a healthy sub-tree on a local drive": {basePath: `C:\Users\someone\.ubuntupro`},
		"a sub-tree on a UNC path": {
			basePath: `\\server\share\publicdir`,
			wantErrs: []error{securefiles.ErrRemoteVolume},
			wantText: "a UNC path",
		},
		// What GetFinalPathNameByHandle hands back for a directory reached through a link
		// into a share: the name the caller used says "C:", so only the resolved form
		// reveals that the sub-tree is not on this machine.
		"a sub-tree resolved to a redirected share": {
			basePath: `\\?\UNC\server\share\publicdir`,
			wantErrs: []error{securefiles.ErrRemoteVolume},
			wantText: "a UNC path",
		},
		"a sub-tree resolved to a local drive": {
			basePath: `\\?\C:\Users\someone\.ubuntupro`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := securefiles.NewCustodianAtBasePath(tc.basePath)

			gaps := c.CheckProjection()
			if len(tc.wantErrs) == 0 {
				require.NoError(t, gaps, "a healthy sub-tree is not a condition worth reporting")
				return
			}

			for _, want := range tc.wantErrs {
				require.ErrorIs(t, gaps, want, "CheckProjection must name every condition that applies")
			}
			if tc.wantText != "" {
				require.ErrorContains(t, gaps, tc.wantText, "CheckProjection must carry what was found")
			}
		})
	}
}

// TestCheckProjectionWithPlatform exercises the resolution step on a custodian that
// actually carries a platform: the sub-tree is classified by where it resolves, not
// where it was named, and a platform-backed root resolves to itself here.
func TestCheckProjectionWithPlatform(t *testing.T) {
	t.Parallel()

	basePath, err := filepath.Abs(t.TempDir())
	require.NoError(t, err, "Setup: could not resolve the temporary directory")

	c, err := securefiles.Open(basePath)
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.CheckProjection(), "a healthy sub-tree on a local volume is not a condition worth reporting")
	require.Equal(t, basePath, c.BasePath())
}
