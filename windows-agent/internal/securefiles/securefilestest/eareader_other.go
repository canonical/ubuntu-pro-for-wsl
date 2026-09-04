//go:build !windows

// Package securefilestest provides test helpers for reading the WSL projection
// extended attributes ($LXUID/$LXGID/$LXMOD) that the Windows custodian stamps
// onto nodes. On Linux the custodian uses a different test watermark, so this
// Windows-specific decoder is a stub.
package securefilestest

import "errors"

// ReadLxAttributes is a stub on non-Windows platforms: it decodes the WSL
// projection EAs written only by the Windows implementation.
func ReadLxAttributes(path string) (uid, gid, mode uint32, err error) {
	return 0, 0, 0, errors.New("ReadLxAttributes is only implemented on Windows")
}
