//go:build windows

package securefiles_test

import (
	"encoding/binary"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// closeHandle closes a Windows handle, swallowing its error return value.
// It is used by test cleanup paths where a close failure is not actionable.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
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
