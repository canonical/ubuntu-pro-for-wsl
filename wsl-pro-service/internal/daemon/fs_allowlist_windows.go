package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowFSNames is a stub on Windows.
// It panics if not running under a test binary.
func AllowFSNames(names ...string) {
	testdetection.MustBeTesting()
}
