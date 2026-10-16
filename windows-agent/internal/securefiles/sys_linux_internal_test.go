//go:build linux

// Unit tests for the Linux watermark internals: the error classifiers that distinguish
// "watermark missing" (not owned) from "xattrs unsupported" (refused, ADR 2.02), driven
// through the swappable syscall hooks that stand in for filesystems this machine lacks.

package securefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestXattrErrorClassification(t *testing.T) {
	t.Parallel()

	require.True(t, isXattrUnsupported(unix.ENOTSUP))
	require.True(t, isXattrUnsupported(unix.EOPNOTSUPP))
	require.False(t, isXattrUnsupported(unix.ENODATA))
	require.False(t, isXattrUnsupported(nil))

	require.True(t, isXattrMissing(unix.ENODATA))
	require.False(t, isXattrMissing(unix.ENOTSUP))
	require.False(t, isXattrMissing(nil))
}

// TestXattrProbeRefusesUnexplainedFailures pins that a probe failing for any reason
// other than "this filesystem cannot carry the watermark" refuses the tree too: an
// unexplained failure is no evidence that stamping works, and the constructor fails
// closed on it rather than handing out a tree it could not establish the capability for.
func TestXattrProbeRefusesUnexplainedFailures(t *testing.T) {
	testCases := map[string]error{
		"an EPERM probe is not a capability answer": unix.EPERM,
		"an EIO probe is not a capability answer":   unix.EIO,
	}
	for name, probeErr := range testCases {
		t.Run(name, func(t *testing.T) {
			openCalls := realXattrCalls()
			openCalls.list = func(int, []byte) (int, error) { return 0, probeErr }

			c, err := openWithXattrs(t.TempDir(), openCalls)
			require.Error(t, err, "an unexplained probe failure must refuse the tree")
			require.Nil(t, c, "no custodian may be handed out on an unexplained probe failure")
			require.NotErrorIs(t, err, ErrNoWatermarkSupport,
				"the refusal must not claim the watermark is unsupported")
		})
	}
}

// TestXattrFailures pins that a filesystem which cannot carry the watermark is refused
// rather than served. The agent's credentials live in this sub-tree, so a sub-tree no
// instance sees as root-owned would publish them to every unprivileged process; refusing
// is the only answer that does not. Failures with any other cause are reported as they are.
func TestXattrFailures(t *testing.T) {
	testCases := map[string]struct {
		// probeErr fails the support probe, before the root is established.
		probeErr error
		// setErr and getErr fail the respective syscall once the root is up.
		setErr error
		getErr error

		// precreate writes a stamped file before the hooks are installed.
		precreate bool

		// op is the operation under test: "write", "isowned", "rename", or "" for none.
		op string

		wantOpenErr bool
		wantErr     bool
	}{
		"a filesystem without xattrs is refused at Open": {
			probeErr:    unix.ENOTSUP,
			wantOpenErr: true,
		},
		// The probe lists attributes on a directory while stamping stores one on a file,
		// so a filesystem can pass the first and refuse the second. The write refuses too.
		"a write the filesystem cannot stamp fails": {
			setErr:  unix.ENOTSUP,
			op:      "write",
			wantErr: true,
		},
		"a write that fails to stamp for another reason fails": {
			setErr:  unix.EPERM,
			op:      "write",
			wantErr: true,
		},
		"isOwned reports a node whose watermark cannot be read": {
			getErr:    unix.ENOTSUP,
			precreate: true,
			op:        "isowned",
			wantErr:   true,
		},
		"isOwned fails when reading the watermark fails for another reason": {
			getErr:    unix.EPERM,
			precreate: true,
			op:        "isowned",
			wantErr:   true,
		},
		// Rename verifies ownership through the same reader, so a node that cannot answer
		// stops the rename rather than being published unverified.
		"rename fails when reading the watermark fails": {
			getErr:    unix.EPERM,
			precreate: true,
			op:        "rename",
			wantErr:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// The probe runs while the root is established, so it must be in place
			// before Open rather than injected afterwards.
			openCalls := realXattrCalls()
			if tc.probeErr != nil {
				openCalls.list = func(int, []byte) (int, error) { return 0, tc.probeErr }
			}

			dir := t.TempDir()
			c, err := openWithXattrs(dir, openCalls)
			if tc.wantOpenErr {
				require.ErrorIs(t, err, ErrNoWatermarkSupport, "a filesystem that cannot carry the watermark must be refused")
				require.Nil(t, c, "no custodian may be handed out for a sub-tree that cannot be secured")
				return
			}
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			if tc.precreate {
				require.NoError(t, c.WriteFile("f.txt", []byte("x")), "Setup: could not write file")
			}

			opCalls := realXattrCalls()
			if tc.setErr != nil {
				opCalls.set = func(int, string, []byte, int) error { return tc.setErr }
			}
			if tc.getErr != nil {
				opCalls.get = func(int, string, []byte) (int, error) { return 0, tc.getErr }
			}
			c.failXattr(opCalls)

			var opErr error
			switch tc.op {
			case "write":
				opErr = c.WriteFile("f.txt", []byte("x"))
			case "isowned":
				_, opErr = c.IsOwned("f.txt")
			case "rename":
				opErr = c.Rename("f.txt", "moved.txt")
			default:
				t.Fatalf("unknown op %q", tc.op)
			}

			if tc.wantErr {
				require.Error(t, opErr, "the operation must not succeed unstamped")
				return
			}
			require.NoError(t, opErr)
		})
	}
}

// TestRenameChecksOwnershipOnLinux pins that a rename moves only nodes the custodian
// still owns. Presence of the watermark is not ownership: an empty, truncated or forged
// value must be refused exactly as a missing one is, otherwise anyone who can write the
// attribute can have the custodian publish their content under a trusted name.
func TestRenameChecksOwnershipOnLinux(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw plants the node behind the custodian's back, so it carries no watermark.
		raw bool
		// dir plants a directory. Directories are never stamped on Linux, so they move
		// on the strength of the sub-tree alone, unlike Windows where they carry 040700.
		dir bool
		// plantWatermark is written as the node's watermark once the node exists, as a
		// tamperer with write access to the attribute would.
		plantWatermark []byte

		wantErr error
	}{
		"a node the custodian wrote": {},

		"a directory": {dir: true},

		"a node written behind its back": {raw: true, wantErr: ErrNotOwned},

		// A watermark that is present but says nothing, or says something that no longer
		// describes the node, proves no more than a missing one.
		"a node with an empty watermark": {
			raw:            true,
			plantWatermark: []byte{},
			wantErr:        ErrNotOwned,
		},
		"a node with a truncated watermark": {
			raw:            true,
			plantWatermark: make([]byte, 8),
			wantErr:        ErrNotOwned,
		},
		"a node whose watermark describes another owner": {
			raw:            true,
			plantWatermark: make([]byte, 12),
			wantErr:        ErrNotOwned,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			const src, dst = "src", "dst"
			switch {
			case tc.dir:
				_, err := c.Subdir(src)
				require.NoError(t, err, "Setup: could not create the sub-tree root")
			case tc.raw:
				require.NoError(t, os.WriteFile(filepath.Join(dir, src), []byte("raw"), 0600),
					"Setup: could not plant the node")
			default:
				require.NoError(t, c.WriteFile(src, []byte("ok")), "Setup: could not write the node")
			}

			if tc.plantWatermark != nil {
				f, err := os.Open(filepath.Join(dir, src))
				require.NoError(t, err, "Setup: could not open the node")
				err = unix.Fsetxattr(int(f.Fd()), watermarkXattr, tc.plantWatermark, 0) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
				require.NoError(t, errors.Join(err, f.Close()), "Setup: could not plant the watermark")
			}

			err = c.Rename(src, dst)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "the rename should have been refused")
				return
			}
			require.NoError(t, err, "the rename should have been allowed")
		})
	}
}

// openWithXattrs opens a custodian over the given xattr surface, so a filesystem that
// refuses or mishandles extended attributes can be stood in for.
func openWithXattrs(basePath string, xattr xattrCalls) (*Custodian, error) {
	return open(basePath, func(p string) (*platformSys, error) { return newPlatformSysWith(p, xattr) })
}

// failXattr replaces the xattr surface of an already-open custodian, for the cases where
// the failure has to begin after the sub-tree has been seeded.
func (c *Custodian) failXattr(xattr xattrCalls) {
	if c.sys == nil {
		return
	}
	c.sys.mu.Lock()
	defer c.sys.mu.Unlock()
	c.sys.xattr = xattr
}

// TestNewPlatformSysRefusesARedirectedRoot pins the containment the Windows
// TestEnsureRootRefusesARedirectedRoot pins: a link standing where the tree root should
// be is refused rather than followed, so the custodian can never be rooted outside the
// path it was given. Open runs before there is a stamped tree to vouch for the
// neighborhood, so a link here is never adopted data.
func TestNewPlatformSysRefusesARedirectedRoot(t *testing.T) {
	testCases := map[string]struct {
		// redirect puts a symlink where the root would be created.
		redirect bool
		// wantFile plants a regular file where the root would be created instead.
		wantFile bool

		wantErr error
	}{
		"a root the custodian creates itself": {},

		"a directory link standing in for the root": {redirect: true, wantErr: ErrPathEscapes},
		"a file standing in for the root":           {wantFile: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")

			rootPath := filepath.Join(base, "root")
			switch {
			case tc.redirect:
				require.NoError(t, os.Symlink(outside, rootPath), "Setup: could not plant the link")
			case tc.wantFile:
				require.NoError(t, os.WriteFile(rootPath, []byte("x"), 0600), "Setup: could not plant the file")
			}

			sys, err := newPlatformSysWith(rootPath, realXattrCalls())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "a redirected root must be refused")
				require.Nil(t, sys, "a refused root must not yield a usable platform")

				entries, err := os.ReadDir(outside)
				require.NoError(t, err, "Setup: could not list the link target")
				require.Empty(t, entries, "nothing may be created through the link")
				return
			}
			if tc.wantFile {
				require.Error(t, err, "a non-directory standing in for the root must be refused")
				require.Nil(t, sys, "a refused root must not yield a usable platform")
				return
			}
			require.NoError(t, err, "the custodian should have established its root")
			require.NoError(t, sys.Close())
		})
	}
}

// TestSetRootRefusesARerootedTree pins the tie between the two resolutions: open()
// re-resolves the path the constructor established, and the parent directory stays
// writable, so a tree swapped in between must be recognized as not the one the
// constructor vouched for.
func TestSetRootRefusesARerootedTree(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "tree")
	sys, err := newPlatformSysWith(rootPath, realXattrCalls())
	require.NoError(t, err, "Setup: could not establish the tree root")

	// Swap the tree: the original directory moves away, and a fresh one stands at
	// the same path. The identity the constructor recorded no longer answers there.
	require.NoError(t, os.Rename(rootPath, filepath.Join(base, "moved")), "Setup: could not move the tree away")
	require.NoError(t, os.Mkdir(rootPath, 0700), "Setup: could not plant the impostor")

	root, err := os.OpenRoot(rootPath)
	require.NoError(t, err, "Setup: could not open the impostor root")
	defer func() { _ = root.Close() }()

	err = sys.setRoot(root)
	require.ErrorIs(t, err, ErrRootReplaced, "a tree swapped in between the resolutions must be refused")
}
