//go:build linux

// Fidelity tests for the Linux xattr watermark — the test-double that mimics
// the Windows EA stamp so ownership and purge logic can be exercised off
// Windows. The double is itself tested here: if it drifts from the Windows
// contract, Linux CI silently stops testing the real adoption policy.

package securefiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
)

func TestLinuxXattrWatermark(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer c.Close()

	// A custodian-written file carries the watermark and reports owned.
	require.NoError(t, c.WriteFile("stamped.txt", []byte("ok")))
	owned, err := c.IsOwned("stamped.txt")
	require.NoError(t, err)
	require.True(t, owned, "custodian-written file should be owned")

	// A raw file lacks the watermark and reports not owned.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "raw.txt"), []byte("raw"), 0600))
	owned, err = c.IsOwned("raw.txt")
	require.NoError(t, err)
	require.False(t, owned, "raw file should not be owned")

	// Directories are never stamped.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o700))
	owned, err = c.IsOwned("subdir")
	require.NoError(t, err)
	require.False(t, owned, "directory should not be owned")

	// Tampering with mode invalidates the watermark.
	require.NoError(t, c.WriteFile("tampered.txt", []byte("ok")))
	//nolint:gosec // G302 - test intentionally widens mode to invalidate the watermark.
	require.NoError(t, os.Chmod(filepath.Join(dir, "tampered.txt"), 0644))
	owned, err = c.IsOwned("tampered.txt")
	require.NoError(t, err)
	require.False(t, owned, "file with changed mode should not be owned")
}
