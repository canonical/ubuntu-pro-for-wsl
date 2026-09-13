//go:build windows

// Package securefilestest provides test helpers for reading the Linux extended attributes
// that the securefiles custodian stamps onto WSL filesystem nodes.
package securefilestest

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procNtQueryEaFile = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryEaFile")

type fileFullEaInformation struct {
	NextEntryOffset uint32
	Flags           uint8
	EaNameLength    uint8
	EaValueLength   uint16
	EaName          [1]byte
}

// ReadLxAttributes reads the $LXUID, $LXGID and $LXMOD extended attributes of the
// node at path and returns their values. The node must exist and be readable.
func ReadLxAttributes(path string) (uid, gid, mode uint32, err error) {
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

	r1, _, _ := procNtQueryEaFile.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&iosb)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&buf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(buf)),
		0, // ReturnSingleEntry = FALSE
		0,
		0,
		0,
		1, // RestartScan = TRUE
	)

	if r1 != 0 {
		return nil, windows.NTStatus(r1) //#nosec G115 // NTSTATUS codes are 32-bit values.
	}

	result := make(map[string]uint32)
	offset := uint32(0)
	for {
		entry := (*fileFullEaInformation)(unsafe.Pointer(&buf[offset])) //#nosec G103 // reinterpreting the kernel-filled EA buffer as its documented header; reads stay within the buffer.
		nameBytes := buf[offset+8 : offset+8+uint32(entry.EaNameLength)]
		name := string(nameBytes)
		valOffset := offset + 8 + uint32(entry.EaNameLength) + 1
		if entry.EaValueLength == 4 {
			val := binary.LittleEndian.Uint32(buf[valOffset : valOffset+4])
			result[name] = val
		}

		if entry.NextEntryOffset == 0 {
			break
		}
		offset += entry.NextEntryOffset
	}

	return result, nil
}
