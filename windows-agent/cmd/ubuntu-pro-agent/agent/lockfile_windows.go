package agent

import (
	"fmt"
	"os"
)

// createLockFile tries to create or open an empty file with given name with exclusive access.
func createLockFile(path string) (*os.File, error) {
	// On Windows removing fails if the file is opened by another process with ERROR_SHARING_VIOLATION.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not remove lock file %s: %v", path, err)
	}
	// If this process is the only instance of this program, then the file won't exist.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not create lock file %s: %v", path, err)
	}

	return f, nil
}
