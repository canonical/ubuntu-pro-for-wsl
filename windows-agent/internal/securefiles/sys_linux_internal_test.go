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
// stamped. Presence of the stamp is not ownership: a raw node, or a value
// the custodian did not write, must be refused exactly as a missing one is,
// otherwise anyone who can write the attribute can have the custodian publish
// their content under a trusted name. The owned happy path itself is pinned
// cross-platform by TestRename.
func TestRenameChecksOwnership(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw plants the node behind the custodian's back, so it carries no stamp.
		raw bool
		// dir creates a directory. Directories carry no file stamp, so they
		// move on the strength of the sub-tree alone.
		dir bool
		// plantStamp is written as the node's stamp once the node
		// exists, as a tamperer with write access to the attribute would.
		plantStamp []byte

		wantErr error
	}{
		// A directory is the other allowance: it carries no file stamp, so it
		// moves on the strength of the sub-tree alone.
		"a directory": {dir: true},

		// A stamp that is present but not the custodian's proves no more
		// than a missing one: presence is not ownership.
		"a node with a truncated stamp": {
			raw:        true,
			plantStamp: make([]byte, 8),
			wantErr:    ErrNotOwned,
		},
		"a node whose stamp names another owner": {
			raw:        true,
			plantStamp: encodeStamp(stamp{uid: 1000, gid: 1000, mode: 0100600}),
			wantErr:    ErrNotOwned,
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
			}

			if tc.plantStamp != nil {
				f, err := os.Open(filepath.Join(dir, src))
				require.NoError(t, err, "Setup: could not open the node")
				err = unix.Fsetxattr(int(f.Fd()), stampXattr, tc.plantStamp, 0) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
				require.NoError(t, errors.Join(err, f.Close()), "Setup: could not plant the stamp")
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

// TestOpenNodeRefusesALink pins the refusal of a link standing at a node
// name: the open fails with ELOOP, which the layer reports as the escape
// sentinel, so a link an instance user planted in the stamped tree is never
// followed to a stamp outside it, and never published through. The redirected
// tree ROOT is pinned by TestNewPlatformSysRefusesARedirectedRoot; this is
// the node level of the same containment. The table covers both positions a
// link can stand at — the leaf and an intermediate component — through the
// operations that resolve names. Each subtest plants its own tree, so the
// rows stay self-contained and parallel.
func TestOpenNodeRefusesALink(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// op resolves a name the way a caller would; the name goes through
		// the planted link, at the leaf or at an intermediate component.
		op func(c *Custodian) error
	}{
		"the ownership check refuses a leaf link": {
			op: func(c *Custodian) error {
				_, err := c.IsOwned("link")
				return err
			},
		},
		"the rename refuses a leaf link": {
			op: func(c *Custodian) error {
				return c.Rename("link", "dst")
			},
		},
		"the ownership check refuses an intermediate link": {
			op: func(c *Custodian) error {
				_, err := c.IsOwned("link/node")
				return err
			},
		},
		"the listing refuses an intermediate link": {
			op: func(c *Custodian) error {
				_, err := c.ReadDir("link/deeper")
				return err
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			outside := filepath.Join(dir, "outside")
			require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")

			c, err := Open(dir)
			require.NoError(t, err, "Setup: could not open the custodian")
			defer func() { _ = c.Close() }()

			require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")), "Setup: could not plant the link")

			require.ErrorIs(t, tc.op(c), ErrPathEscapes, "resolving through the link must be refused, not followed")
		})
	}
}

// TestOpenNodeBackstopAnswersTheContractError pins the one refusal the
// custodian's own lexical checks should never let reach the layer: an
// escaping "..". RESOLVE_BENEATH answers EXDEV, and the classification must
// turn it into the escape sentinel — a raw EXDEV is an errno no caller
// matches, so resolve() regressing must still fail closed with the contract
// error.
func TestOpenNodeBackstopAnswersTheContractError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err, "Setup: could not open the root")
	defer func() { _ = root.Close() }()

	s := newPlatformSys(dir)
	require.NoError(t, s.attach(root), "Setup: could not attach the layer")
	defer func() { _ = s.release() }()

	_, err = s.openNode("..", openRead)
	require.ErrorIs(t, err, ErrPathEscapes, "RESOLVE_BENEATH's backstop must answer the escape sentinel, not a raw EXDEV")
}

// TestNewPlatformSysRefusesARedirectedRoot pins the containment Open rests on:
// a link standing where the tree root should be is refused rather than
// followed, so the custodian can never be rooted outside the path it was
// given. Open runs before there is a stamped tree to vouch for the
// neighborhood, so a link here is never adopted data.
func TestNewPlatformSysRefusesARedirectedRoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")

	rootPath := filepath.Join(base, "root")
	require.NoError(t, os.Symlink(outside, rootPath), "Setup: could not plant the link")

	sys := newDoubleSysWith(rootPath, realStampCalls())
	_, _, err := sys.establishRoot()
	require.ErrorIs(t, err, ErrPathEscapes, "a redirected root must be refused")

	entries, err := os.ReadDir(outside)
	require.NoError(t, err, "Setup: could not list the link target")
	require.Empty(t, entries, "nothing may be created through the link")
}
func TestStampsRefuseUnstampableTrees(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// probeErr fails the support probe, before the root is established.
		probeErr error
		// wantUnsupported asserts the refusal names ErrNoStampSupport,
		// as a filesystem without xattrs does; anything else must not claim it.
		wantUnsupported bool
	}{
		"a filesystem without xattrs is refused at Open": {probeErr: unix.ENOTSUP, wantUnsupported: true},
		"an EPERM probe is not a capability answer":      {probeErr: unix.EPERM},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stamps := realStampCalls()
			stamps.list = func(int) error { return tc.probeErr }

			c, err := openWithStamps(t.TempDir(), stamps)
			require.Error(t, err, "the tree must be refused rather than handed out")
			require.Nil(t, c, "no custodian may be handed out for a refused tree")
			if tc.wantUnsupported {
				require.ErrorIs(t, err, ErrNoStampSupport, "the refusal must name the stamp as unsupported")
			} else {
				require.NotErrorIs(t, err, ErrNoStampSupport,
					"the refusal must not claim the stamp is unsupported")
			}
		})
	}
}

// TestStampRefusalsAreClassified pins what a stamping failure means on the
// create path: a failure that says the filesystem cannot carry the stamp
// is reclassified to ErrNoStampSupport, while any other failure surfaces
// as itself — the caller must not have to guess which is which. Both carry
// the same cleanup: the fresh node does not stay behind unstamped. The
// support probe that refuses an unsupported filesystem at Open is pinned by
// TestStampsRefuseUnstampableTrees; reaching the stamp here means the probe
// settled and the filesystem changed its mind under us.
func TestStampRefusalsAreClassified(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		setErr error

		// wantUnsupported asserts the failure is answered as a filesystem
		// that cannot carry the stamp; anything else must surface as
		// itself.
		wantUnsupported bool
	}{
		"a filesystem without xattr support is named as such": {setErr: unix.ENOTSUP, wantUnsupported: true},
		"a failing filesystem surfaces its own error":         {setErr: unix.EIO},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stamps := realStampCalls()
			stamps.set = func(int, []byte) error { return tc.setErr }
			dir := t.TempDir()

			c, err := openWithStamps(dir, stamps)
			require.NoError(t, err, "Setup: the tree must establish; only the node stamping fails")
			defer func() { _ = c.Close() }()

			err = c.WriteFile("f", []byte("x"))
			require.Error(t, err, "a node that cannot be stamped must not be served")
			if tc.wantUnsupported {
				require.ErrorIs(t, err, ErrNoStampSupport, "the refusal must name the stamp as unsupported")
			} else {
				require.NotErrorIs(t, err, ErrNoStampSupport, "the failure must surface as itself")
			}

			// The cleanup the refusal owes, on both classifications.
			require.NoFileExists(t, filepath.Join(dir, "f"), "the refused node must not stay behind unstamped")
		})
	}
}

// TestStampReadFailuresAreClassified pins the read side of the same boundary:
// a stamp that cannot be read is not a verdict about the node, so the failure
// surfaces; one that says the filesystem cannot carry the stamp is
// reclassified so a caller can recognise it, and reaching the read at all
// means the support probe settled at Open. The no-stamp verdict ENODATA
// answers is pinned by the tampered-node cases of TestRenameChecksOwnership.
func TestStampReadFailuresAreClassified(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		getErr error

		wantUnsupported bool
	}{
		"an unsupported get is reclassified": {getErr: unix.ENOTSUP, wantUnsupported: true},
		"a failing get surfaces as itself":   {getErr: unix.EIO},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// The node is stamped by a healthy custodian first, because the
			// write verifies its own stamp and would refuse just as rightly
			// as the read below does. The tree survives the custodian — the
			// stamp is on disk — so the failing one adopts it.
			dir := t.TempDir()
			seed, err := openWithStamps(dir, realStampCalls())
			require.NoError(t, err, "Setup: could not open the seeding custodian")
			require.NoError(t, seed.WriteFile("f", []byte("x")), "Setup: could not write the stamped node")
			require.NoError(t, seed.Close(), "Setup: could not close the seeding custodian")

			stamps := realStampCalls()
			stamps.get = func(int, []byte) (int, error) { return 0, tc.getErr }

			c, err := openWithStamps(dir, stamps)
			require.NoError(t, err, "Setup: the tree must adopt")
			defer func() { _ = c.Close() }()

			_, err = c.IsOwned("f")
			require.Error(t, err, "a stamp that cannot be read is not a verdict about the node")
			if tc.wantUnsupported {
				require.ErrorIs(t, err, ErrNoStampSupport, "the failure must be recognisable as unsupported")
			} else {
				require.NotErrorIs(t, err, ErrNoStampSupport, "the failure must surface as itself")
			}
		})
	}
}

// openWithStamps opens a custodian over the given stamping surface, so a
// filesystem that refuses or mishandles the stamp can be stood in for.
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
