//go:build windows

package securefiles_test

import (
	"encoding/binary"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// closeHandle closes a Windows handle, swallowing its error return value.
// It is used by test cleanup paths where a close failure is not actionable.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
}

// holdReparsePointOpen holds the reparse point itself (never its target) open without
// delete-sharing, so nothing can remove it while the test runs. It pins the refusal
// behind the sub-tree heal: a link that cannot be removed refuses the sub-tree.
func holdReparsePointOpen(t *testing.T, path string) {
	t.Helper()

	path16, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err, "Setup: could not name the planted link")

	h, err := windows.CreateFile(
		path16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		t.Skipf("holding the link open not permitted in this environment: %v", err)
	}
	t.Cleanup(func() { closeHandle(h) })
}

// lxAttributes builds the $LXUID/$LXGID/$LXMOD attributes themselves, so a test table can
// declare the stamp it plants instead of encoding it in the test body.
func lxAttributes(uid, gid, mode uint32) []winio.ExtendedAttribute {
	var uidBytes, gidBytes, modeBytes [4]byte
	binary.LittleEndian.PutUint32(uidBytes[:], uid)
	binary.LittleEndian.PutUint32(gidBytes[:], gid)
	binary.LittleEndian.PutUint32(modeBytes[:], mode)

	return []winio.ExtendedAttribute{
		{Name: "$LXUID", Value: uidBytes[:]},
		{Name: "$LXGID", Value: gidBytes[:]},
		{Name: "$LXMOD", Value: modeBytes[:]},
	}
}
