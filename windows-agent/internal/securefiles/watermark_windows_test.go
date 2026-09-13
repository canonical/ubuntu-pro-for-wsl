//go:build windows

// Ownership-predicate unit tests over real extended attributes, symmetric to
// watermark_linux_test.go: while the stamping mechanism is verified in
// sys_windows_test.go, this file verifies what IsOwned makes of it. In
// particular it pins that directories are never adopted: they ARE stamped
// (040700) but the predicate only accepts the regular-file stamp (0100600) —
// an agreement with the Linux watermark (which never stamps directories) that
// rides on the file-type bits inside stampedFileMode().

package securefiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestWindowsEaWatermark(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw writes the node behind the custodian's back, so it carries no attributes
		// at all. Windows has no empty-list answer, so the query itself fails.
		raw bool
		// subdir creates the node as a sub-tree root. Sub-tree roots are stamped 040700 —
		// verified in sys_windows_test.go — yet must never be adopted here: the predicate
		// accepts only the regular-file stamp.
		subdir bool
		// rewriteEa replaces the node's attributes after it is written, standing in for
		// an instance taking ownership, or for a stamp that decodes but proves nothing.
		rewriteEa []winio.ExtendedAttribute
		// degraded marks the filesystem as unable to carry the stamp before the query.
		degraded  bool
		wantOwned bool
		wantErr   bool
	}{
		"a node the custodian wrote": {wantOwned: true},

		"a node written behind its back": {raw: true, wantErr: true},

		// Note the mechanism divergence with Linux, where directories are never stamped
		// and IsOwned reports (false, nil): here the predicate opens with
		// FILE_NON_DIRECTORY_FILE, so a directory errors instead. Either way, never owned.
		"a sub-tree root": {subdir: true, wantErr: true},

		"a node whose stamp was rewritten": {rewriteEa: lxAttributes(1000, 1000, 0100640)},

		// An incomplete stamp decodes fine, so the predicate reports a clean "not owned"
		// and reserves errors for nodes whose query fails. These must be planted on raw
		// nodes: the custodian stamps its own writes completely, and the attribute write
		// merges rather than replaces.
		"a node stamped with only $LXUID": {
			raw:       true,
			rewriteEa: []winio.ExtendedAttribute{{Name: "$LXUID", Value: []byte{0, 0, 0, 0}}},
		},
		"a node whose $LXMOD is too short": {
			raw: true,
			rewriteEa: []winio.ExtendedAttribute{
				{Name: "$LXUID", Value: []byte{0, 0, 0, 0}},
				{Name: "$LXGID", Value: []byte{0, 0, 0, 0}},
				{Name: "$LXMOD", Value: []byte{0, 0}},
			},
		},

		// Degradation is not an answer about ownership. A filesystem that cannot carry
		// the attributes leaves every node unverifiable, so the predicate keeps reporting
		// the query failure instead of adopting the node: deciding that an unverifiable
		// sub-tree is "ours" is the caller's policy, and pinning it here keeps that policy
		// from drifting back into the platform layer.
		"an unstamped node, degraded": {raw: true, degraded: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			const node = "node"
			switch {
			case tc.subdir:
				sub, err := c.Subdir(node)
				require.NoError(t, err, "Setup: could not create the sub-tree root")
				require.NoError(t, sub.Close(), "Setup: could not close the sub-custodian")
			case tc.raw:
				require.NoError(t, os.WriteFile(filepath.Join(dir, node), []byte("raw"), 0600),
					"Setup: could not plant the node")
			default:
				require.NoError(t, c.WriteFile(node, []byte("ok")), "Setup: could not write the node")
			}

			if tc.rewriteEa != nil {
				h, err := openForEaWrite(filepath.Join(dir, node))
				require.NoError(t, err, "Setup: could not open the node for an attribute write")
				buf, err := winio.EncodeExtendedAttributes(tc.rewriteEa)
				require.NoError(t, err, "Setup: could not encode the attributes")
				require.NoError(t, setEaFile(h, buf), "Setup: could not rewrite the attributes")
				closeHandle(h)
			}

			if tc.degraded {
				c.SetDegraded(true)
			}

			owned, err := c.IsOwned(node)
			if tc.wantErr {
				require.Error(t, err, "the predicate should have reported an unreadable stamp")
			} else {
				require.NoError(t, err, "the stamp should have decoded cleanly")
			}
			require.Equal(t, tc.wantOwned, owned, "unexpected ownership answer")
		})
	}
}

// setEaFile replaces the extended attributes of the node behind h with the
// encoded EA list, simulating WSL taking ownership of a node.
func setEaFile(h windows.Handle, eaBuf []byte) error {
	var iosb windows.IO_STATUS_BLOCK
	return windows.NtSetEaFile(h, &iosb, &eaBuf[0], uint32(len(eaBuf)) /* #nosec G115 */)
}

// openForEaWrite opens path with the sharing and access flags needed to write
// its extended attributes, for tests that plant raw EA payloads.
func openForEaWrite(path string) (windows.Handle, error) {
	return windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
}
