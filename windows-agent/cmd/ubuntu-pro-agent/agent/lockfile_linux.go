package agent

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// createLockFile tries to create or open an empty file with given name with exclusive access.
// If the file already exists AND is still locked, it will fail.
func createLockFile(path string) (*os.File, error) {
	// This would fail if the file is locked by another process.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not create lock file %s: %v", path, err)
	}
	// This would only fail if the file is locked by another process.
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return nil, fmt.Errorf("could not lock file %s: %v", path, errors.Join(err, file.Close()))
	}

	return file, nil
}
