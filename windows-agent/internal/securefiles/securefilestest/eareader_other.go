//go:build !windows

// Package securefilestest provides test helpers for reading the Linux extended attributes
// that the securefiles custodian stamps onto WSL filesystem nodes.
package securefilestest

import "errors"

// ReadLxAttributes is a stub on non-Windows platforms because Linux extended attributes
// are only stamped by the securefiles custodian on Windows.
func ReadLxAttributes(path string) (uid, gid, mode uint32, err error) {
	return 0, 0, 0, errors.New("ReadLxAttributes is only implemented on Windows")
}
