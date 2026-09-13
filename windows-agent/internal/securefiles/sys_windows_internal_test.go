//go:build windows

// Internal tests over the Windows platform layer: they drive the NT syscall hooks to
// reproduce filesystems this machine does not have, and pin ADR 2.02 throughout — a
// refusal degrades the custodian loudly and never fails closed.

package securefiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestOpenStampingFailureDegrades(t *testing.T) {
	testCases := map[string]struct {
		// preCreate leaves the root on disk before Open, so it is adopted rather than created.
		preCreate bool

		wantDegraded bool
	}{
		"a pre-existing root, stamped separately": {preCreate: true, wantDegraded: true},
		"a root created by ensureRoot itself":     {},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			rootDir := filepath.Join(t.TempDir(), ".cloud-init")
			if tc.preCreate {
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")
			}

			hook := test.NewGlobal()
			defer hook.Reset()

			// Force NtSetEaFile to fail as if the filesystem denied the EA write.
			denied := uint32(windows.STATUS_ACCESS_DENIED)
			testNtSetEaFileResult = &denied
			defer func() { testNtSetEaFileResult = nil }()

			cust, err := Open(rootDir)
			require.NoError(t, err, "stamping must never fail closed")
			defer cust.Close()

			// A root ensureRoot creates carries its stamp in the NtCreateFile call itself
			// (ADR 2.01), so it never reaches the attribute write this hook denies. Only
			// an adopted root is stamped separately, and only it can degrade here.
			require.Equal(t, tc.wantDegraded, cust.IsDegraded(), "unexpected degraded state")
			require.Equal(t, tc.wantDegraded, loggedAt(hook, logrus.ErrorLevel, ""),
				"the condition must be reported exactly when it occurs")
		})
	}
}

// TestCreateFileEaFailureDegradesAndFallsBack drives the createNode failure paths
// through the testNtCreateFileResult hook: an EA-rejection must degrade the
// custodian and fall back to plain creation, while an unrelated failure surfaces
// as an error.
func TestCreateNode(t *testing.T) {
	testCases := map[string]struct {
		// failCreation makes NtCreateFile fail with this status, standing in for a
		// filesystem that rejects the extended attributes carried on the create.
		failCreation uint32

		wantErr      bool
		wantExists   bool
		wantDegraded bool
	}{
		"an EA rejection falls back to plain creation": {
			failCreation: uint32(windows.STATUS_EAS_NOT_SUPPORTED),
			wantExists:   true, wantDegraded: true,
		},
		"an unrelated failure surfaces as an error": {
			failCreation: uint32(windows.STATUS_ACCESS_DENIED),
			wantErr:      true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// The hook is read inside createNode only, so Open still establishes a real root.
			status := tc.failCreation
			testNtCreateFileResult = &status
			defer func() { testNtCreateFileResult = nil }()

			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			const node = "node.txt"
			err = cust.WriteFile(node, []byte("data"))
			if tc.wantErr {
				require.Error(t, err, "a non-EA creation failure must surface as an error")
			} else {
				require.NoError(t, err, "an EA-rejected creation must fail open into the plain fallback")
			}

			path := filepath.Join(dir, node)
			if tc.wantExists {
				require.FileExists(t, path, "the fallback must still create the node plainly")
			} else {
				require.NoFileExists(t, path, "a failed creation must leave no node behind")
			}
			require.Equal(t, tc.wantDegraded, cust.IsDegraded(), "unexpected degraded state")
		})
	}
}

// TestRemoteVolumeClassification pins how the public directory's volume is classified.
// The distinction matters because a remote volume is the one case where the custodian
// reports healthy while the guarantee may not hold: stamping succeeds on the server, but
// what instances see is a projection of a share. UNC paths must be recognised without
// calling GetDriveType, which blocks on name resolution for an unreachable server and
// then reports DRIVE_NO_ROOT_DIR regardless.
func TestRemoteVolumeClassification(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path string

		wantRemote bool
		wantKind   string
	}{
		"a local drive":                 {path: `C:\Users\someone\.ubuntupro`},
		"a relative path has no volume": {path: `.ubuntupro`},
		"a UNC path":                    {path: `\\server\share\Users\someone\.ubuntupro`, wantRemote: true, wantKind: "a UNC path"},
		"an unreachable UNC path":       {path: `\\no-such-host-xyz\share\.ubuntupro`, wantRemote: true, wantKind: "a UNC path"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()
			remote, kind := remoteVolume(tc.path)
			elapsed := time.Since(start)

			require.Equal(t, tc.wantRemote, remote, "volume should be classified as remote=%v", tc.wantRemote)
			require.Equal(t, tc.wantKind, kind)

			// Classification runs on the startup path, so it must never reach out to
			// the network: an unreachable server would otherwise stall the agent.
			require.Less(t, elapsed, 250*time.Millisecond, "classification must not perform a network lookup")
		})
	}
}

// TestSetRootVerifiesIdentity covers the gap between creating the root and opening it.
// ensureRoot creates and stamps the directory through a handle, closes it, and setRoot
// then resolves the same path again; os.Root does not protect its own root, and ADR 2.01
// concedes the parent stays writable from inside an instance. If the path is redirected
// in between, every later "secure" write lands in the attacker's directory, stamped and
// reported as owned. Identity is the only thing tying the two resolutions together.
func TestSetRootVerifiesIdentity(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// replaced opens a different directory than the one ensureRoot prepared,
		// standing in for a path redirected between the two resolutions.
		replaced bool
		// unverifiable models a filesystem that supplies no usable file identity,
		// such as a network redirector.
		unverifiable bool

		wantErr error
	}{
		"the directory that was created":      {},
		"a directory swapped in behind it":    {replaced: true, wantErr: ErrRootReplaced},
		"no identity to compare against":      {unverifiable: true},
		"swapped, but no identity to compare": {replaced: true, unverifiable: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			rootDir := filepath.Join(base, "root")

			sys, err := newPlatformSys(rootDir)
			require.NoError(t, err, "Setup: could not establish the root")
			require.True(t, sys.rootIDKnown, "Setup: the local filesystem should supply an identity")

			opened := rootDir
			if tc.replaced {
				opened = filepath.Join(base, "elsewhere")
				require.NoError(t, os.MkdirAll(opened, 0700), "Setup: could not create the swapped directory")
			}
			if tc.unverifiable {
				sys.rootIDKnown = false
			}

			root, err := os.OpenRoot(opened)
			require.NoError(t, err, "Setup: could not open the root")
			defer func() { _ = root.Close() }()

			err = sys.setRoot(root)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "a redirected root must be refused")
				return
			}
			require.NoError(t, err, "setRoot should accept the root")
			require.NoError(t, sys.Close())
		})
	}
}

// loggedAt reports whether the hook captured an entry at level whose message contains
// substr. An empty substr matches any message at that level.
func loggedAt(hook *test.Hook, level logrus.Level, substr string) bool {
	for _, entry := range hook.AllEntries() {
		if entry.Level == level && strings.Contains(entry.Message, substr) {
			return true
		}
	}
	return false
}

// TestEnsureRootRefusesARedirectedRoot pins that a directory link standing where the root
// should be is refused rather than followed. OBJ_DONT_REPARSE makes the creation fail on
// purpose; the degraded fallback must not undo that, because os.MkdirAll succeeds on an
// existing directory link and that path records no identity, leaving setRoot nothing to
// compare and the custodian rooted outside basePath in silence.
func TestEnsureRootRefusesARedirectedRoot(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// redirect puts a directory symlink where the root would be created, standing in
		// for any reparse point: a junction planted from inside an instance behaves alike.
		redirect bool

		wantErr error
	}{
		"a root the custodian creates itself": {},

		"a directory link standing in for the root": {redirect: true, wantErr: ErrPathEscapes},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			require.NoError(t, os.MkdirAll(outside, 0700), "Setup: could not create the link target")

			rootPath := filepath.Join(base, "root")
			if tc.redirect {
				if err := os.Symlink(outside, rootPath); err != nil {
					t.Skip("symlink creation not permitted in this environment")
				}
			}

			sys, err := newPlatformSys(rootPath)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "a redirected root must be refused")
				require.Nil(t, sys, "a refused root must not yield a usable platform")

				entries, err := os.ReadDir(outside)
				require.NoError(t, err, "Setup: could not list the link target")
				require.Empty(t, entries, "nothing may be created through the link")
				return
			}
			require.NoError(t, err, "the custodian should have established its root")
			require.NoError(t, sys.Close())
		})
	}
}
