package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowFSTypes is a stub on Windows.
// It panics if not running under a test binary.
func AllowFSTypes(types ...int64) {
	testdetection.MustBeTesting()
}
