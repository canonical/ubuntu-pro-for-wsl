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
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

var procNtSetEaFile = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetEaFile")

// setEaFile replaces the extended attributes of the node behind h with the
// encoded EA list, simulating WSL taking ownership of a node.
func setEaFile(h windows.Handle, eaBuf []byte) error {
	var iosb windows.IO_STATUS_BLOCK
	r1, _, _ := procNtSetEaFile.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&iosb)),     //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(eaBuf)),
	)
	if r1 != 0 {
		return windows.NTStatus(r1) //#nosec G115 // NTSTATUS codes are 32-bit values.
	}
	return nil
}

func TestWindowsEaWatermark(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	// A custodian-written file carries the stamp and reports owned.
	require.NoError(t, c.WriteFile("stamped.txt", []byte("ok")))
	owned, err := c.IsOwned("stamped.txt")
	require.NoError(t, err)
	require.True(t, owned, "custodian-written file should be owned")

	// A raw file carries no EAs at all: the EA query itself fails (Windows has
	// no empty-list answer), so the predicate reports an error. Either way the
	// node is never owned.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "raw.txt"), []byte("raw"), 0600))
	owned, err = c.IsOwned("raw.txt")
	require.Error(t, err, "IsOwned on an EA-less file errors on Windows")
	require.False(t, owned, "raw file should not be owned")

	// A directory created via Subdir is stamped (040700) yet must not be
	// adopted: the predicate only accepts the regular-file stamp (0100600).
	// Note the mechanism divergence with Linux, where directories are never
	// stamped and IsOwned reports (false, nil): on Windows the predicate opens
	// with FILE_NON_DIRECTORY_FILE, so a directory errors instead. Either way
	// a directory is never reported owned.
	sub, err := c.Subdir("subdir")
	require.NoError(t, err)
	require.NoError(t, sub.Close())
	uid, gid, mode, err := securefilestest.ReadLxAttributes(filepath.Join(dir, "subdir"))
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(040700), mode)
	owned, err = c.IsOwned("subdir")
	require.Error(t, err, "IsOwned on a directory errors on Windows (FILE_NON_DIRECTORY_FILE)")
	require.False(t, owned, "directory should never be owned")

	// Rewriting the EAs as if WSL took ownership or changed permissions
	// invalidates the stamp.
	require.NoError(t, c.WriteFile("tampered.txt", []byte("ok")))
	h, err := openForEaWrite(filepath.Join(dir, "tampered.txt"))
	require.NoError(t, err)
	eaBuf, err := encodeLxEa(1000, 1000, 0100640)
	require.NoError(t, err)
	require.NoError(t, setEaFile(h, eaBuf))
	closeHandle(h)

	owned, err = c.IsOwned("tampered.txt")
	require.NoError(t, err)
	require.False(t, owned, "file with rewritten EAs should not be owned")

	// An incomplete stamp (only $LXUID) or a wrongly-sized value (a 2-byte
	// $LXMOD) cannot prove ownership either, but both decode fine: the
	// predicate reports a clean "not owned", reserving errors for files whose
	// EA query itself fails. These must be planted on raw files: the custodian
	// stamps its own writes completely, and NtSetEaFile merges rather than
	// replaces the EA list.
	partials := map[string][]winio.ExtendedAttribute{
		"partial.txt":  {{Name: "$LXUID", Value: []byte{0, 0, 0, 0}}},
		"shortmod.txt": {{Name: "$LXUID", Value: []byte{0, 0, 0, 0}}, {Name: "$LXGID", Value: []byte{0, 0, 0, 0}}, {Name: "$LXMOD", Value: []byte{0, 0}}},
	}
	for name, eas := range partials {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("ok"), 0600))
		h, err := openForEaWrite(filepath.Join(dir, name))
		require.NoError(t, err)
		eaBuf, err := winio.EncodeExtendedAttributes(eas)
		require.NoError(t, err)
		require.NoError(t, setEaFile(h, eaBuf))
		closeHandle(h)

		owned, err := c.IsOwned(name)
		require.NoError(t, err, "incomplete stamps should decode without error")
		require.False(t, owned, "incomplete stamp should not be owned")
	}
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
