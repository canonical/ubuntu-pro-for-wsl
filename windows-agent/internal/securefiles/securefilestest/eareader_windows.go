//go:build windows

// Package securefilestest provides test helpers for reading the Linux extended attributes
// that the securefiles custodian stamps onto WSL filesystem nodes.
package securefilestest

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

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

	if err := windows.NtQueryEaFile(handle, &iosb, &buf[0], uint32(len(buf)) /* #nosec G115 */, false, nil, 0, nil, true); err != nil {
		var status windows.NTStatus
		if errors.As(err, &status) {
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
