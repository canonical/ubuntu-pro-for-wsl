package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowTestOwner permits an additional UID and GID during testing (e.g. for mock agents
// running as an unprivileged test user).
// It panics if not running under a test binary.
//
// This function must only be called during package initialization (init())
// in test helpers (such as testutils) before any concurrent test execution starts.
func AllowTestOwner(uid, gid uint32) {
	testdetection.MustBeTesting()

	expectedUID = uid
	expectedGID = gid
}
