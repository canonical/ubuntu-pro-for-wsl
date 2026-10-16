//go:build windows

// Windows-only: a concurrent reader holding the file open with full sharing
// (including FILE_SHARE_DELETE) must never observe a partially written file
// during the custodian's atomic rename. Not reproducible on Linux, where
// rename semantics and open locking differ.

package securefiles_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestRenameOverOpenDestination pins the publishing guarantee: a rename that replaces
// a file readers still hold open must go through, because that is the whole reason the
// custodian publishes by rename rather than by writing in place. It only holds with
// POSIX rename semantics; the information class that predates them refuses the rename
// whenever the destination is open, however politely the reader shares it.
//
// A reader that withholds FILE_SHARE_DELETE is refused, and refused immediately: that
// is a defect in the reader, not a transient condition, so waiting cannot resolve it.
// The elapsed-time bound is what keeps a retry loop from creeping back in.
func TestRenameOverOpenDestination(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// politeReader holds the destination open sharing deletion, as any reader of
		// a file published by rename must.
		politeReader bool

		wantErr bool
	}{
		"a rename replaces a destination held by a polite reader": {politeReader: true},
		"a rename is refused by a reader withholding deletion":    {wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			require.NoError(t, c.WriteFile("source.txt", []byte("new")), "Setup: could not seed source")
			require.NoError(t, c.WriteFile("target.txt", []byte("old")), "Setup: could not seed target")

			holder, err := holdDestination(filepath.Join(dir, "target.txt"), tc.politeReader)
			require.NoError(t, err, "Setup: could not hold the destination open")
			defer closeHandle(holder)

			start := time.Now()
			err = c.Rename("source.txt", "target.txt")
			elapsed := time.Since(start)

			// Whatever the outcome, it is reached without waiting: neither replacing an
			// open destination nor refusing an unshareable one is a transient condition.
			require.Less(t, elapsed, 500*time.Millisecond, "rename should not wait on the destination")

			if tc.wantErr {
				require.Error(t, err, "a reader withholding deletion must refuse the rename")
				require.FileExists(t, filepath.Join(dir, "source.txt"), "a refused rename must leave the source in place")
				return
			}

			require.NoError(t, err, "rename should replace a destination held by a polite reader")
			got, err := os.ReadFile(filepath.Join(dir, "target.txt"))
			require.NoError(t, err)
			require.Equal(t, "new", string(got), "destination should hold the renamed content")
			require.NoFileExists(t, filepath.Join(dir, "source.txt"), "source should be gone after a successful rename")

			// The reader that was already holding the destination keeps reading the
			// content it opened: the replacement unlinks the old node, never mutates it.
			stale, err := io.ReadAll(&handleReader{h: holder})
			require.NoError(t, err, "the pre-existing handle should still be readable")
			require.Equal(t, "old", string(stale), "an open reader must not observe the replacement through its own handle")
		})
	}
}

type handleReader struct {
	h windows.Handle
}

func (r *handleReader) Read(p []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(r.h, p, &n, nil)
	if errors.Is(err, windows.ERROR_HANDLE_EOF) || (n == 0 && err == nil) {
		return int(n), io.EOF
	}
	if err != nil {
		return int(n), err
	}
	return int(n), nil
}

// holdDestination opens path the way a reader of a rename-published file does, or,
// when shareDelete is false, the way a reader that breaks that contract does.
func holdDestination(path string, shareDelete bool) (windows.Handle, error) {
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if shareDelete {
		share |= windows.FILE_SHARE_DELETE
	}

	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}

	return windows.CreateFile(
		path16,
		windows.GENERIC_READ,
		share,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
}

// TestCreateFileAnswersAHeldNodeWithTheSentinel pins the real classification
// the sentinel rests on: a host-side process holding the node without sharing
// deletion (an ordinary os.Open shares read and write only) makes the
// replacement answer ErrNodeHeld, and the hold lifting lets the replacement
// through. This is the mechanism the daemon's address-file retry waits on.
func TestCreateFileAnswersAHeldNodeWithTheSentinel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	require.NoError(t, c.WriteFile("target.txt", []byte("OLD")), "Setup: could not seed the node")

	holder, err := os.Open(filepath.Join(dir, "target.txt"))
	require.NoError(t, err, "Setup: could not hold the node open")
	defer func() { _ = holder.Close() }()

	_, err = c.CreateFile("target.txt")
	require.ErrorIs(t, err, securefiles.ErrNodeHeld, "a holder without delete sharing must answer the held sentinel")

	require.NoError(t, holder.Close(), "Setup: could not lift the hold")

	f, err := c.CreateFile("target.txt")
	require.NoError(t, err, "the replacement must go through once the hold is lifted")
	_, err = f.WriteString("NEW")
	require.NoError(t, err, "the write must succeed")
	require.NoError(t, f.Close(), "the replacement must close cleanly")

	got, err := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, err)
	require.Equal(t, "NEW", string(got), "the node must hold the replacement's content")
}

// TestNodeLevelLinksAreRefused pins the node-level refusal against a real
// symlink on the real filesystem: a link standing at a node name is never
// followed, whatever operation asks. The status-to-sentinel translation is
// pinned by TestMapNtStatus, and the tree-root level by the establishRoot
// tests; this is the ordinary-node level of the same containment, exercised
// end to end through the public operations.
func TestNodeLevelLinksAreRefused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err, "Setup: could not open custodian")
	defer func() { _ = c.Close() }()

	// A link planted inside the tree, pointing outside it. It cannot be
	// planted through the custodian; planting it behind its back is exactly
	// what the refusal answers for.
	target := filepath.Join(filepath.Dir(dir), "outside-secret.txt")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0600), "Setup: could not create the link target")
	t.Cleanup(func() { _ = os.Remove(target) })
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "link")), "Setup: could not plant the link")
	// And one pointing inside, for the in-tree flavor of the same refusal.
	require.NoError(t, c.WriteFile("inside.txt", []byte("inside")), "Setup: could not seed an in-tree node")
	require.NoError(t, os.Symlink("inside.txt", filepath.Join(dir, "inlink")), "Setup: could not plant the in-tree link")

	_, err = c.IsOwned("link")
	require.ErrorIs(t, err, securefiles.ErrPathEscapes, "the ownership predicate must refuse a link standing at the name, not follow it")

	_, err = c.IsOwned("inlink")
	require.ErrorIs(t, err, securefiles.ErrPathEscapes, "an in-tree link is refused as surely as an escaping one: the predicate answers for the node, not the target")

	err = c.Rename("link", "renamed")
	require.ErrorIs(t, err, securefiles.ErrPathEscapes, "the rename must refuse a link standing at the source name, not publish through it")

	// The link target is untouched by every refusal above.
	got, err := os.ReadFile(target)
	require.NoError(t, err, "the link target must still be readable under its own name")
	require.Equal(t, "secret", string(got), "no refusal may reach through the link to its target")
}
