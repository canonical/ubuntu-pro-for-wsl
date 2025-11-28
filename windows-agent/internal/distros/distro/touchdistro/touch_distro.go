package touchdistro

import (
	"errors"
)

type wslDistroNotFoundError struct {
	err error
}

func (e *wslDistroNotFoundError) Error() string {
	return e.err.Error()
}

// IsWslDistroNotFound tells whether an error chain contains an error caused by attempting to touch
// a distro instance that no longer exists.
func IsWslDistroNotFound(err error) bool {
	var e *wslDistroNotFoundError
	return errors.As(err, &e)
}
