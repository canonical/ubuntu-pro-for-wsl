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

// encodeLxEa builds the $LXUID/$LXGID/$LXMOD extended-attribute buffer WSL
// uses to project Linux metadata, with the given values.
func encodeLxEa(uid, gid uint32, mode uint32) ([]byte, error) {
	var uidBytes, gidBytes, modeBytes [4]byte
	binary.LittleEndian.PutUint32(uidBytes[:], uid)
	binary.LittleEndian.PutUint32(gidBytes[:], gid)
	binary.LittleEndian.PutUint32(modeBytes[:], mode)

	eas := []winio.ExtendedAttribute{
		{Name: "$LXUID", Value: uidBytes[:]},
		{Name: "$LXGID", Value: gidBytes[:]},
		{Name: "$LXMOD", Value: modeBytes[:]},
	}
	return winio.EncodeExtendedAttributes(eas)
}
