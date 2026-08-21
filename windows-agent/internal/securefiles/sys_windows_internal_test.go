//go:build windows

// Internal test driving the real EA-write failure path through the
// testNtSetEaFileResult hook: a stamping failure must mark the custodian
// degraded (fail-open), never fail closed.

package securefiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestOpenPreExistingRootStampingFailureMarksDegraded(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, ".cloud-init")

	// Pre-create the root without stamping.
	require.NoError(t, os.MkdirAll(rootDir, 0750))

	hook := test.NewGlobal()
	defer hook.Reset()

	// Force NtSetEaFile to fail as if the filesystem denied the EA write.
	denied := uint32(windows.STATUS_ACCESS_DENIED)
	testNtSetEaFileResult = &denied
	defer func() { testNtSetEaFileResult = nil }()

	cust, err := Open(rootDir)
	require.NoError(t, err, "Open should succeed even when EA stamping fails")
	defer cust.Close()

	require.True(t, cust.IsDegraded(), "Custodian should be degraded when EA stamping fails")

	foundError := false
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.ErrorLevel {
			foundError = true
			break
		}
	}
	require.True(t, foundError, "Expected error-level log message when stamping fails")
}

// TestCreateFileEaFailureDegradesAndFallsBack drives the createNode failure paths
// through the testNtCreateFileResult hook: an EA-rejection must degrade the
// custodian and fall back to plain creation, while an unrelated failure surfaces
// as an error.
func TestCreateFileEaFailureDegradesAndFallsBack(t *testing.T) {
	// Force NtCreateFile in createNode to fail as if the filesystem rejected EAs:
	// creation must fail open into the plain fallback and mark the custodian degraded.
	denied := uint32(windows.STATUS_EAS_NOT_SUPPORTED)
	testNtCreateFileResult = &denied
	defer func() { testNtCreateFileResult = nil }()

	func() {
		dir := t.TempDir()
		cust, err := Open(dir)
		require.NoError(t, err, "Setup: could not open custodian")
		defer cust.Close()

		require.NoError(t, cust.WriteFile("unstamped.txt", []byte("data")),
			"EA-rejected creation should fail open into the plain fallback")
		require.True(t, cust.IsDegraded(), "custodian should be degraded after an EA-unsupported failure")
		require.FileExists(t, filepath.Join(dir, "unstamped.txt"), "fallback should still create the file plainly")
	}()

	// An unrelated failure is not an EA problem: it must surface as an error.
	// This needs a fresh custodian: a degraded one never reaches NtCreateFile.
	hard := uint32(windows.STATUS_ACCESS_DENIED)
	testNtCreateFileResult = &hard

	func() {
		dir := t.TempDir()
		cust, err := Open(dir)
		require.NoError(t, err, "Setup: could not open custodian")
		defer cust.Close()

		require.Error(t, cust.WriteFile("blocked.txt", []byte("data")),
			"non-EA creation failures should surface as errors")
		require.False(t, cust.IsDegraded(), "non-EA failure must not mark the custodian degraded")
		require.NoFileExists(t, filepath.Join(dir, "blocked.txt"), "failed creation must leave no file behind")
	}()
}
