// Package testutils implements helper functions for frequently needed functionality
// in tests.
package testutils

import (
	"testing"
)

// Backup stores the current value of a variable, and restores it at test cleanup.
func Backup[T any](t *testing.T, ptr *T) {
	t.Helper()

	backUp := *ptr
	t.Cleanup(func() {
		*ptr = backUp
	})
}

// DefineInjector is syntax sugar to facilitate dependency injection. Use it as such:
//
//		// In the module
//		var foo = func(/* etc */) (/* etc */) { /* etc */ }
//
//	    // In the module's export_test.go
//		var InjectFoo = testutils.DefineInjector(&foo)
//
//		// In the tests
//		injectFoo(t, fooMock)
func DefineInjector[T any](f *T) func(*testing.T, T) {
	return func(t *testing.T, g T) {
		t.Helper()
		Backup(t, f)
		*f = g
	}
}
