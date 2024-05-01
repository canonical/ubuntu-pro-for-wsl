// Package testdetection helps in deciding if we are currently running under tests.
package testdetection

import (
	"testing"
)

var integrationtests = false

// MustBeTesting panics if we are not running under tests.
func MustBeTesting() {
	if !testing.Testing() && !integrationtests {
		panic("This can only be called in tests")
	}
}
