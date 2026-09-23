package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// AllowFSTypes extends the allowed filesystem types for testing.
// It panics if not running under a test binary.
//
// This function must only be called during package initialization (init())
// in test helpers (such as testutils) before any concurrent test execution starts.
func AllowFSTypes(types ...int64) {
	testdetection.MustBeTesting()

	for _, t := range types {
		allowedFSTypes[t] = "test-allowed"
	}
}
