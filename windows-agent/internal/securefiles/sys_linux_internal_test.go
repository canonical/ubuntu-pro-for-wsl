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
			c, err := OpenWithXattrs(dir, openCalls)
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
			c.FailXattr(opCalls)

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
