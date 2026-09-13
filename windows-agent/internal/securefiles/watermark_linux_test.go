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
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
)

func TestLinuxXattrWatermark(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// raw writes the node behind the custodian's back, so it carries no watermark.
		raw bool
		// dir creates a directory instead of a file; directories are never stamped.
		dir bool
		// chmod widens the mode after the write, invalidating the watermark.
		chmod bool
		// degraded marks the filesystem as unable to carry the watermark before the query.
		degraded bool

		wantOwned bool
	}{
		"a node the custodian wrote":     {wantOwned: true},
		"a node written behind its back": {raw: true},
		"a directory":                    {dir: true},
		"a node whose mode was changed":  {chmod: true},
		"a stamped node, degraded":       {degraded: true, wantOwned: true},
		"an unstamped node, degraded":    {raw: true, degraded: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			const node = "node"
			switch {
			case tc.dir:
				require.NoError(t, os.Mkdir(filepath.Join(dir, node), 0o700), "Setup: could not create the directory")
			case tc.raw:
				require.NoError(t, os.WriteFile(filepath.Join(dir, node), []byte("raw"), 0600), "Setup: could not plant the node")
			default:
				require.NoError(t, c.WriteFile(node, []byte("ok")), "Setup: could not write the node")
			}

			if tc.chmod {
				//nolint:gosec // G302 - the test intentionally widens the mode to invalidate the watermark.
				require.NoError(t, os.Chmod(filepath.Join(dir, node), 0644), "Setup: could not change the mode")
			}

			// Degradation is not an answer about ownership, mirroring the Windows
			// predicate: a filesystem that cannot carry the watermark leaves every node
			// judged on what it still carries, and deciding what that means is the
			// caller's policy, not this predicate's.
			if tc.degraded {
				c.SetDegraded(true)
			}

			owned, err := c.IsOwned(node)
			require.NoError(t, err, "the Linux predicate reports a missing watermark cleanly")
			require.Equal(t, tc.wantOwned, owned, "unexpected ownership answer")
		})
	}
}

// TestProjectionReaderIsWindowsOnly has a single case by construction: off Windows the
// projection attributes cannot exist, so there is only one answer to pin. The two
// watermarks are different mechanisms, and the Windows reader must refuse rather than
// fabricate the root-owned values the Windows tests assert on.
func TestProjectionReaderIsWindowsOnly(t *testing.T) {
	t.Parallel()

	_, _, _, err := securefilestest.ReadLxAttributes(t.TempDir())
	require.Error(t, err, "the WSL projection attributes must not be readable off Windows")
}
