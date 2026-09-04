package daemon

import (
	"errors"
	"io/fs"
)

// wsl-pro-service only ever runs inside WSL (Linux). The definition below exists
// solely so the package keeps compiling for cross-platform development tooling.
// It fails closed: no file can prove root ownership on this platform.

// ownershipOf always fails on Windows: ownership cannot be proven, so validation must refuse.
func ownershipOf(info fs.FileInfo) (uid, gid uint32, err error) {
	return 0, 0, errors.New("ownership metadata is not available on Windows")
}

func openRootOS(path string) (rootFs, error) {
	return nil, errors.New("openRootOS is not implemented on Windows")
}
