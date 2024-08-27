package microsoftstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// findWorkspaceRoot climbs up the current working directory until the Go workspace root is found.
func findWorkspaceRoot() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", errors.New("could not get current working directory")
	}

	for {
		parent := filepath.Dir(path)
		if parent == path {
			// Reached filesystem root
			return "", errors.New("could not find repository root")
		}
		path = parent

		if s, err := os.Stat(filepath.Join(path, "go.work")); err == nil && !s.IsDir() {
			return parent, nil
		}
	}
}

// checkError inspects the values of hres and err to determine what kind of error we have, if any, according to the rules of syscall/dll_windows.go.
func checkError(hres int64, err error) (int64, error) {
	// From syscall/dll_windows.go (*Proc).Call doc:
	// > Callers must inspect the primary return value to decide whether an
	//   error occurred [...] before consulting the error.
	if e := NewStoreAPIError(hres); e != nil {
		return hres, fmt.Errorf("storeApi returned error code %d: %w", hres, e)
	}

	if err == nil {
		return hres, nil
	}

	var target syscall.Errno
	if b := errors.As(err, &target); !b {
		// Supposedly unreachable: proc.Call must always return a syscall.Errno
		return hres, err
	}

	if target != syscall.Errno(0) {
		return hres, fmt.Errorf("failed syscall to storeApi: %v (syscall errno %d)", target, err)
	}

	return hres, nil
}
