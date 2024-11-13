package agent

import (
	"os"

	"github.com/ubuntu/decorate"
)

// createLockFile tries to create or open an empty file with given name with exclusive access.
func createLockFile(path string) (f *os.File, err error) {
	decorate.OnError(&err, "could not create lock file %s: %v", path, err)

	// On Windows removing fails if the file is opened by another process with ERROR_SHARING_VIOLATION.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// If this process is the only instance of this program, then the file won't exist.
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0600)
}
