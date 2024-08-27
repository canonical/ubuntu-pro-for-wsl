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
	// There is no possibility of nil  error, the `err` return value is always constructed with the
	// result of `GetLastError()` which could have been set by something completely
	// unrelated to our code some time in the past, as well as it could be `ERROR_SUCCESS` which is the `Errno(0)`.
	// If the act of calling the API fails (not the function we're calling, but the attempt to call it), then we'd
	// have a meaningful `syscall.Errno` object via the `err` parameter, related to the actual failure (like a function not found in this DLL)
	// Since our implementation of the store API doesn't touch errno the call should return `hres`
	// in our predefined range plus garbage in the `err` argument, thus we only care about the `hres` in this case.
	if e := NewStoreAPIError(hres); e != nil {
		return hres, fmt.Errorf("storeApi returned error code %d: %w", hres, e)
	}

	// Supposedly unreachable: proc.Call must always return a non-nil syscall.Errno
	if err == nil {
		return hres, nil
	}

	var target syscall.Errno
	if b := errors.As(err, &target); !b {
		// Supposedly unreachable: proc.Call must always return a non-nil syscall.Errno
		return hres, err
	}

	// The act of calling our API didn't succeed, function not found in the DLL for example:
	if target != syscall.Errno(0) {
		return hres, fmt.Errorf("failed syscall to storeApi: %v (syscall errno %d)", target, err)
	}

	// A non-error value in hres plus ERROR_SUCCESS in err.
	// This shouldn't happen in the current store API implementation anyway.
	return hres, nil
}
