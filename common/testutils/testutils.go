// Package testutils implements helper functions for frequently needed functionality
// in tests.
package testutils

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// ReplaceFileWithDir removes a file and creates a directory with the same path.
// Useful to break file reads and assert on the errors.
func ReplaceFileWithDir(t *testing.T, path string, msg string, args ...any) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil {
		err = fmt.Errorf("could not remove file: %v", err)
		require.NoErrorf(t, err, msg, args...)
	}

	if err := os.MkdirAll(path, 0700); err != nil {
		err = fmt.Errorf("could not create folder at file's location: %v", err)
		require.NoErrorf(t, err, msg, args...)
	}
}
