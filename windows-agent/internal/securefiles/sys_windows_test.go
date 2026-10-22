//go:build windows

// Windows production-mechanism tests over real extended attributes: what the
// custodian stamps with $LXUID/$LXGID/$LXMOD and when (TestWindowsStamping),
// and what the ownership predicate makes of what it stamped (TestIsOwned).
// The stamping is read through readLxAttributes below, an instrument that shares
// nothing with the custodian but winio's decoder: this external test package
// cannot reach the custodian's own stamp reading, so a stamping bug cannot
// certify itself;
// the predicate verdicts themselves are single-sourced in custodian.go and
// tested cross-platform over the fake platformSys — this file pins the real
// EA shapes those verdicts rest on: a stamp that decodes but proves nothing
// is not ownership, and stamps that decode only partially answer a clean
// "not owned", reserving errors for nodes whose query fails.

package securefiles_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestWindowsStamping pins what carries the $LXUID/$LXGID/$LXMOD stamp, which is what
// projects a node as root-owned inside an instance. Files are stamped as they are created
// and survive being published by rename. Directories are the one place ADR 2.01 allows
// stamping in place: the public root and first-level sub-tree roots are adopted when an
// earlier run left them behind, and their stamp is what revokes unprivileged creation and
// deletion inside them. What the predicate makes of these is pinned by TestIsOwned, below.
func TestWindowsStamping(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// subdir names a sub-tree root reached through Subdir rather than the root itself.
		subdir string
		// preCreate plants the directories with plain os.MkdirAll before the custodian
		// exists, so they are adopted and stamped in place rather than created stamped.
		preCreate bool
		// file names a regular file written through the custodian.
		file string
		// renameTo publishes that file under a new name before the stamp is read.
		renameTo string

		wantMode uint32
	}{
		"a created root":           {wantMode: 040700},
		"an adopted root":          {preCreate: true, wantMode: 040700},
		"a created sub-tree root":  {subdir: "certs", wantMode: 040700},
		"an adopted sub-tree root": {subdir: "certs", preCreate: true, wantMode: 040700},
		"a created file":           {file: "file.txt", wantMode: 0100600},
		"a published file":         {file: "file.txt", renameTo: "file.old", wantMode: 0100600},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := filepath.Join(t.TempDir(), "root")
			if tc.preCreate {
				plant := rootDir
				if tc.subdir != "" {
					plant = filepath.Join(rootDir, tc.subdir)
				}
				require.NoError(t, os.MkdirAll(plant, 0750), "Setup: could not plant the directory")
			}

			cust, err := securefiles.Open(rootDir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = cust.Close() }()

			stamped := rootDir
			if tc.subdir != "" {
				sub, err := cust.Subdir(tc.subdir)
				require.NoError(t, err, "the sub-tree root must be usable")
				defer func() { _ = sub.Close() }()
				stamped = filepath.Join(rootDir, tc.subdir)
			}
			if tc.file != "" {
				require.NoError(t, cust.WriteFile(tc.file, []byte("content")), "the file must be written")
				stamped = filepath.Join(rootDir, tc.file)
			}
			if tc.renameTo != "" {
				require.NoError(t, cust.Rename(tc.file, tc.renameTo), "an owned node must be publishable")
				stamped = filepath.Join(rootDir, tc.renameTo)
			}

			uid, gid, mode, err := readLxAttributes(stamped)
			require.NoError(t, err, "the node must carry the stamp")
			require.Equal(t, uint32(0), uid, "the stamp must claim root as owner")
			require.Equal(t, uint32(0), gid, "the stamp must claim root as group")
			require.Equal(t, tc.wantMode, mode, "unexpected mode in the stamp")
		})
	}
}

// TestIsOwned pins what the ownership predicate makes of the attributes the
// real filesystem carries — the shapes the fake platformSys cannot produce.
// A stamp another writer left through a real attribute write decodes but
// proves nothing; a stamp that decodes only partially answers a clean
// "not owned", reserving errors for nodes whose query fails — the other side
// of that boundary is pinned by TestReadStampSurfacesAnUndecodableStamp.
func TestIsOwned(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw plants the node with plain os.WriteFile, so it carries no
		// custodian stamp for the rewrite to merge into.
		raw bool
		// rewriteEa replaces the node's attributes after it is written, standing in for
		// an instance taking ownership, or for a stamp that decodes but proves nothing.
		rewriteEa []winio.ExtendedAttribute
		wantOwned bool
	}{
		"a node the custodian wrote": {wantOwned: true},

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

			owned, err := c.IsOwned(node)
			require.NoError(t, err, "the stamp should have answered cleanly")
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

// readLxAttributes reads the $LXUID, $LXGID and $LXMOD extended attributes of
// the node at path and returns their values. The node must exist and be
// readable. It is the independent instrument the stamping tests read through:
// it goes to the raw attribute buffer via winio's decoder, never through the
// custodian's stamp reading, which this external test package cannot reach.
func readLxAttributes(path string) (uid, gid, mode uint32, err error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not convert path to UTF-16: %v", err)
	}

	h, err := windows.CreateFile(
		path16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not open %q: %v", path, err)
	}
	defer func() { _ = windows.CloseHandle(h) }()

	eas, err := ntQueryAllEa(h)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not query EAs for %q: %v", path, err)
	}

	uid, ok := eas["$LXUID"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("missing $LXUID extended attribute on %q", path)
	}
	gid, ok = eas["$LXGID"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("missing $LXGID extended attribute on %q", path)
	}
	mode, ok = eas["$LXMOD"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("missing $LXMOD extended attribute on %q", path)
	}

	return uid, gid, mode, nil
}

func ntQueryAllEa(handle windows.Handle) (map[string]uint32, error) {
	var iosb windows.IO_STATUS_BLOCK
	buf := make([]byte, 2048)

	if err := windows.NtQueryEaFile(handle, &iosb, &buf[0], uint32(len(buf)) /* #nosec G115 */, false, nil, 0, nil, true); err != nil {
		if status, ok := errors.AsType[windows.NTStatus](err); ok {
			return nil, status
		}
		return nil, err
	}

	decoded, err := winio.DecodeExtendedAttributes(buf[:iosb.Information])
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint32)
	for _, ea := range decoded {
		if len(ea.Value) == 4 {
			result[ea.Name] = binary.LittleEndian.Uint32(ea.Value)
		}
	}

	return result, nil
}
