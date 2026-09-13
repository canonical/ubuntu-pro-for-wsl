//go:build windows

// Windows production-mechanism tests: files are stamped with
// $LXUID/$LXGID/$LXMOD at creation, the root is stamped when adopting a
// pre-existing directory, and rename targets are stamped. What the ownership
// predicate makes of those EAs is unit-tested in watermark_windows_test.go.

package securefiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
)

func TestAtomicCreationStampingOnWindows(t *testing.T) {
	dir := t.TempDir()

	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer c.Close()

	// File creation carries owner 0, group 0, mode 0100600.
	err = c.WriteFile("file.txt", []byte("hello"))
	require.NoError(t, err)

	uid, gid, mode, err := securefilestest.ReadLxAttributes(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(0100600), mode)

	// The root directory itself carries owner 0, group 0, mode 040700.
	uid, gid, mode, err = securefilestest.ReadLxAttributes(dir)
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(040700), mode)
}

func TestOpenPreExistingUnstampedRootStampsEA(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "pre-existing-root")

	// Pre-create directory via plain os.MkdirAll (unstamped).
	require.NoError(t, os.MkdirAll(rootDir, 0750))

	// Open via custodian.
	cust, err := securefiles.Open(rootDir)
	require.NoError(t, err)
	defer cust.Close()

	require.False(t, cust.IsDegraded(), "Custodian should stamp pre-existing root and not be degraded")

	// Verify EA attributes and values on the root.
	uid, gid, mode, err := securefilestest.ReadLxAttributes(rootDir)
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(040700), mode)
}

func TestRenameStampsEAOnUnstampedNode(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	// Create custodian
	cust, err := securefiles.Open(rootDir)
	require.NoError(t, err)
	defer cust.Close()

	// Seed raw unstamped file using plain os.WriteFile
	rawPath := filepath.Join(rootDir, "raw.log")
	require.NoError(t, os.WriteFile(rawPath, []byte("raw content"), 0600))

	// Rotate via custodian.Rename
	err = cust.Rename("raw.log", "raw.old")
	require.NoError(t, err)

	// Verify EA attributes were stamped on the renamed target node
	uid, gid, mode, err := securefilestest.ReadLxAttributes(filepath.Join(rootDir, "raw.old"))
	require.NoError(t, err)
	require.Equal(t, uint32(0), uid)
	require.Equal(t, uint32(0), gid)
	require.Equal(t, uint32(0100600), mode)
}
