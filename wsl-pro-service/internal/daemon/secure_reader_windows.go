package daemon

import (
	"errors"
)

// wsl-pro-service only ever runs inside WSL (Linux). The definition below exists
// solely so the package keeps compiling for cross-platform development tooling.
// It fails closed: no file can prove root ownership on this platform.

func openRootOS(path string) (rootFs, error) {
	return nil, errors.New("openRootOS is not implemented on Windows")
}
