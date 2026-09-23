package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowTestOwner is a stub on Windows.
// It panics if not running under a test binary.
func AllowTestOwner(uid, gid uint32) {
	testdetection.MustBeTesting()
}
