//go:build windows

// Pins the Windows ReadLxAttributes contract: every failure mode of the EA
// reading pipeline (unconvertible path, unopenable node, failed EA query,
// incomplete or wrongly-shaped stamp) must surface as an error rather than
// fabricated ownership metadata.

package securefilestest_test

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

var procNtSetEaFile = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetEaFile")

func TestReadLxAttributes(t *testing.T) {
	dir := t.TempDir()

	// A complete, well-formed stamp round-trips.
	stamped := filepath.Join(dir, "stamped.txt")
	writeWithEas(t, stamped, []winio.ExtendedAttribute{
		{Name: "$LXUID", Value: []byte{0, 0, 0, 0}},
		{Name: "$LXGID", Value: []byte{0, 0, 0, 0}},
		{Name: "$LXMOD", Value: []byte{0x80, 0x81, 0x00, 0x00}}, // 0100600 = 0x8180, little-endian
	})
	uid, gid, mode, err := securefilestest.ReadLxAttributes(stamped)
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(0100600), mode)

	t.Run("path with a NUL byte cannot be converted", func(t *testing.T) {
		_, _, _, err := securefilestest.ReadLxAttributes("bad\x00path")
		require.Error(t, err)
	})

	t.Run("missing node cannot be opened", func(t *testing.T) {
		_, _, _, err := securefilestest.ReadLxAttributes(filepath.Join(dir, "nonexistent"))
		require.Error(t, err)
	})

	t.Run("EA-less node fails the query outright", func(t *testing.T) {
		raw := filepath.Join(dir, "raw.txt")
		require.NoError(t, os.WriteFile(raw, []byte("raw"), 0600))
		_, _, _, err := securefilestest.ReadLxAttributes(raw)
		require.Error(t, err, "Windows has no empty-list answer to the EA query")
	})

	partialSets := map[string][]winio.ExtendedAttribute{
		"only $LXMOD and $LXGID, missing $LXUID": {
			{Name: "$LXGID", Value: []byte{0, 0, 0, 0}},
			{Name: "$LXMOD", Value: []byte{0, 0, 0, 0}},
		},
		"only $LXUID, missing $LXGID": {
			{Name: "$LXUID", Value: []byte{0, 0, 0, 0}},
		},
		"$LXUID and $LXGID, missing $LXMOD": {
			{Name: "$LXUID", Value: []byte{0, 0, 0, 0}},
			{Name: "$LXGID", Value: []byte{0, 0, 0, 0}},
		},
		"wrongly-sized $LXMOD is skipped, missing $LXMOD": {
			{Name: "$LXUID", Value: []byte{0, 0, 0, 0}},
			{Name: "$LXGID", Value: []byte{0, 0, 0, 0}},
			{Name: "$LXMOD", Value: []byte{0, 0}},
		},
	}
	for name, eas := range partialSets {
		t.Run(name, func(t *testing.T) {
			node := filepath.Join(dir, "partial.txt")
			writeWithEas(t, node, eas)
			_, _, _, err := securefilestest.ReadLxAttributes(node)
			require.Error(t, err, "incomplete stamps must error rather than fabricate values")
			require.NoError(t, os.Remove(node))
		})
	}
}

// writeWithEas creates path with the given extended attributes planted raw.
func writeWithEas(t *testing.T, path string, eas []winio.ExtendedAttribute) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))

	eaBuf, err := winio.EncodeExtendedAttributes(eas)
	require.NoError(t, err)

	h, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	require.NoError(t, err)
	defer func() { _ = windows.CloseHandle(h) }()

	var iosb windows.IO_STATUS_BLOCK
	r1, _, _ := procNtSetEaFile.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&iosb)),     //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(eaBuf)),
	)
	require.Zero(t, r1, "NtSetEaFile failed with status %#x", r1)
}
