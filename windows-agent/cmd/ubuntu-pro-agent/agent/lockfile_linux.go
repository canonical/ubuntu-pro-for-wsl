package agent

import (
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"

	"github.com/ubuntu/decorate"
)

// createLockFile tries to create or open an empty file with given name with exclusive access.
// If the file already exists AND is still locked, it will fail.
func createLockFile(path string) (f *os.File, err error) {
	decorate.OnError(&err, "could not create lock file %s: %v", path, err)

	f, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
		}
	}()
	fd := f.Fd()
	if fd > math.MaxInt {
		return nil, fmt.Errorf("file descriptor %d exceeds maximum integer value", fd)
	}
	// This would only fail if the file is locked by another process.
	err = syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return nil, fmt.Errorf("could not lock file: %v", err)
	}

	return f, nil
}
