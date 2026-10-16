//go:build linux

// Contract tests for the Linux double: the platform-independent
// guarantees the custodian rests on — a rename moves only nodes the
// custodian stamped, and a tree root the custodian did not establish itself
// is refused rather than redirected into. The stamping mechanism itself is a
// means, not a guarantee, and is not tested here; the guarantees the Windows
// mechanisms deliver are pinned by the Windows-only tests.

package securefiles

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestRenameChecksOwnership pins that a rename moves only nodes the custodian
// stamped. Presence of the watermark is not ownership: a raw node, or a value
// the custodian did not write, must be refused exactly as a missing one is,
// otherwise anyone who can write the attribute can have the custodian publish
// their content under a trusted name.
func TestRenameChecksOwnership(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw plants the node behind the custodian's back, so it carries no watermark.
		raw bool
		// dir creates a directory. Directories carry no file stamp, so they
		// move on the strength of the sub-tree alone.
		dir bool
		// plantWatermark is written as the node's watermark once the node
		// exists, as a tamperer with write access to the attribute would.
		plantWatermark []byte

		wantErr error
	}{
		"a node the custodian wrote": {},

		"a directory": {dir: true},

		// A watermark that is present but not the custodian's proves no more
		// than a missing one: presence is not ownership.
		"a node with a truncated watermark": {
			raw:            true,
			plantWatermark: make([]byte, 8),
			wantErr:        ErrNotOwned,
		},
		"a node whose watermark names another owner": {
			raw:            true,
			plantWatermark: encodeStamp(stamp{uid: 1000, gid: 1000, mode: 0100600}),
			wantErr:        ErrNotOwned,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := Open(dir)
			require.NoError(t, err, "Setup: could not open the custodian")
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

// TestNewPlatformSysRefusesARedirectedRoot pins the containment Open rests on:
// a link standing where the tree root should be is refused rather than
// followed, so the custodian can never be rooted outside the path it was
// given. Open runs before there is a stamped tree to vouch for the
// neighborhood, so a link here is never adopted data.
func TestNewPlatformSysRefusesARedirectedRoot(t *testing.T) {
	testCases := map[string]struct {
		// redirect puts a symlink where the root would be created.
		redirect bool
	}{
		"a root the custodian creates itself":       {},
		"a directory link standing in for the root": {redirect: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")

			rootPath := filepath.Join(base, "root")
			if tc.redirect {
				require.NoError(t, os.Symlink(outside, rootPath), "Setup: could not plant the link")
			}

			sys := newDoubleSysWith(rootPath, realStampCalls())
			_, _, err := sys.establishRoot()
			if tc.redirect {
				require.ErrorIs(t, err, ErrPathEscapes, "a redirected root must be refused")

				entries, err := os.ReadDir(outside)
				require.NoError(t, err, "Setup: could not list the link target")
				require.Empty(t, entries, "nothing may be created through the link")
				return
			}
			require.NoError(t, err, "the custodian should have established its root")
		})
	}
}

// TestStampsRefuseUnstampableTrees pins the refuse-at-open policy (ADR 2.02)
// against the double: a filesystem that cannot carry the watermark is refused
// rather than served, and an unexplained probe failure is refused too, rather
// than taken as evidence that stamping works.
func TestStampsRefuseUnstampableTrees(t *testing.T) {
	testCases := map[string]struct {
		// probeErr fails the support probe, before the root is established.
		probeErr error
		// wantUnsupported asserts the refusal names ErrNoWatermarkSupport,
		// as a filesystem without xattrs does; anything else must not claim it.
		wantUnsupported bool
	}{
		"a filesystem without xattrs is refused at Open": {probeErr: unix.ENOTSUP, wantUnsupported: true},
		"an EPERM probe is not a capability answer":      {probeErr: unix.EPERM},
		"an EIO probe is not a capability answer":        {probeErr: unix.EIO},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			stamps := realStampCalls()
			stamps.list = func(int) error { return tc.probeErr }

			c, err := openWithStamps(t.TempDir(), stamps)
			require.Error(t, err, "the tree must be refused rather than handed out")
			require.Nil(t, c, "no custodian may be handed out for a refused tree")
			if tc.wantUnsupported {
				require.ErrorIs(t, err, ErrNoWatermarkSupport, "the refusal must name the watermark as unsupported")
			} else {
				require.NotErrorIs(t, err, ErrNoWatermarkSupport,
					"the refusal must not claim the watermark is unsupported")
			}
		})
	}
}

// openWithStamps opens a custodian over the given stamping surface, so a
// filesystem that refuses or mishandles the watermark can be stood in for.
func openWithStamps(basePath string, stamps stampCalls) (*Custodian, error) {
	return open(basePath, func(p string) platformSys { return newDoubleSysWith(p, stamps) })
}

// encodeStamp encodes the stamp the way every carrier does: the owner, group
// and mode, each a little-endian uint32 — the values the Windows layer writes
// as NT extended attributes and the double as a user xattr.
func encodeStamp(st stamp) []byte {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[0:4], st.uid)
	binary.LittleEndian.PutUint32(b[4:8], st.gid)
	binary.LittleEndian.PutUint32(b[8:12], st.mode)
	return b[:]
}
