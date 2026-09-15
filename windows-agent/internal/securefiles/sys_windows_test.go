//go:build windows

// Windows production-mechanism tests: what the custodian stamps with
// $LXUID/$LXGID/$LXMOD, and when. What the ownership predicate makes of those
// attributes is unit-tested in watermark_windows_test.go.

package securefiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
)

// TestWindowsStamping pins what carries the $LXUID/$LXGID/$LXMOD stamp, which is what
// projects a node as root-owned inside an instance. Files are stamped as they are created
// and survive being published by rename. Directories are the one place ADR 2.01 allows
// stamping in place: the public root and first-level sub-tree roots are adopted when an
// earlier run left them behind, and their stamp is what revokes unprivileged creation and
// deletion inside them. What the predicate makes of these is in watermark_windows_test.go.
func TestWindowsStamping(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// subdir names a sub-tree root reached through Subdir rather than the root itself.
		subdir string
		// preCreate plants the directories with plain os.MkdirAll before the custodian
		// exists, so they are adopted and stamped in place rather than created stamped.
		preCreate bool
		// file names a regular file written through the custodian.
		file string
		// renameTo publishes that file under a new name before the stamp is read.
		renameTo string

		wantMode uint32
	}{
		"a created root":           {wantMode: 040700},
		"an adopted root":          {preCreate: true, wantMode: 040700},
		"a created sub-tree root":  {subdir: "certs", wantMode: 040700},
		"an adopted sub-tree root": {subdir: "certs", preCreate: true, wantMode: 040700},
		"a created file":           {file: "file.txt", wantMode: 0100600},
		"a published file":         {file: "file.txt", renameTo: "file.old", wantMode: 0100600},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := filepath.Join(t.TempDir(), "root")
			if tc.preCreate {
				plant := rootDir
				if tc.subdir != "" {
					plant = filepath.Join(rootDir, tc.subdir)
				}
				require.NoError(t, os.MkdirAll(plant, 0750), "Setup: could not plant the directory")
			}

			cust, err := securefiles.Open(rootDir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = cust.Close() }()

			stamped := rootDir
			if tc.subdir != "" {
				sub, err := cust.Subdir(tc.subdir)
				require.NoError(t, err, "the sub-tree root must be usable")
				defer func() { _ = sub.Close() }()
				stamped = filepath.Join(rootDir, tc.subdir)
			}
			if tc.file != "" {
				require.NoError(t, cust.WriteFile(tc.file, []byte("content")), "the file must be written")
				stamped = filepath.Join(rootDir, tc.file)
			}
			if tc.renameTo != "" {
				require.NoError(t, cust.Rename(tc.file, tc.renameTo), "an owned node must be publishable")
				stamped = filepath.Join(rootDir, tc.renameTo)
			}

			require.False(t, cust.IsDegraded(), "stamping a usable filesystem must not degrade the custodian")

			uid, gid, mode, err := securefilestest.ReadLxAttributes(stamped)
			require.NoError(t, err, "the node must carry the stamp")
			require.Equal(t, uint32(0), uid, "the stamp must claim root as owner")
			require.Equal(t, uint32(0), gid, "the stamp must claim root as group")
			require.Equal(t, tc.wantMode, mode, "unexpected mode in the stamp")
		})
	}
}
