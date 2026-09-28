package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowFSNames extends the allowed filesystem names for testing.
// It panics if not running under a test binary.
//
// This function must only be called during package initialization (init())
// in test helpers (such as testutils) before any concurrent test execution starts.
func AllowFSNames(names ...string) {
	testdetection.MustBeTesting()

	for _, name := range names {
		allowedFSNames[name] = true
	}
}
