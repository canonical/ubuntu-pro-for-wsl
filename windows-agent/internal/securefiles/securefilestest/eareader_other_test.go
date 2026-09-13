//go:build !windows

// Pins the non-Windows stub contract: the WSL projection extended attributes
// exist only on Windows, so ReadLxAttributes must error rather than fabricate
// values.

package securefilestest_test

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
)

func TestReadLxAttributesStub(t *testing.T) {
	t.Parallel()

	_, _, _, err := securefilestest.ReadLxAttributes(t.TempDir())
	require.Error(t, err)
}
