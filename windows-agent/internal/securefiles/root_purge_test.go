// Cross-platform root lifecycle and purge policy: Open adopts (creating and
// stamping if needed) the root, and Purge removes unrecognised nodes and
// leftover temporaries by caller policy. The root stamp itself is asserted
// via securefilestest.ReadLxAttributes on Windows only.

package securefiles_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	"github.com/stretchr/testify/require"
)

func TestRootPurge(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		prepare func(t *testing.T, parent string) string
		makeSub bool
		preseed func(t *testing.T, c *securefiles.Custodian)
		run     func(t *testing.T, c *securefiles.Custodian) []string
		check   func(t *testing.T, c *securefiles.Custodian, removed []string)
	}{
		"Open establishes and stamps a root directory": {
			prepare: func(t *testing.T, parent string) string {
				t.Helper()
				return filepath.Join(parent, "publicdir")
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				basePath := c.BasePath()
				require.Empty(t, removed)
				fi, err := os.Stat(basePath)
				require.NoError(t, err)
				require.True(t, fi.IsDir())

				if runtime.GOOS == "windows" {
					uid, gid, mode, err := securefilestest.ReadLxAttributes(basePath)
					require.NoError(t, err, "could not read root directory extended attributes")
					require.Equal(t, uint32(0), uid, "root directory should be owned by root")
					require.Equal(t, uint32(0), gid, "root directory should be group root")
					require.Equal(t, uint32(040700), mode, "root directory should have mode 0700")
				}
			},
		},
		"Purge removes unrecognised nodes and temp files while keeping allowed ones": {
			prepare: func(t *testing.T, parent string) string {
				t.Helper()
				return parent
			},
			preseed: func(t *testing.T, c *securefiles.Custodian) {
				t.Helper()
				basePath := c.BasePath()
				require.NoError(t, c.WriteFile("legit.txt", []byte("valid")))
				require.NoError(t, os.WriteFile(filepath.Join(basePath, "malicious.txt"), []byte("bad"), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(basePath, ".tmp-legit.txt-1234"), []byte("tmp"), 0600))
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				allowedPolicy := func(relPath string) bool { return relPath == "legit.txt" }
				removed, err := c.Purge(allowedPolicy)
				require.NoError(t, err)
				return removed
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				basePath := c.BasePath()
				require.Contains(t, removed, "malicious.txt")
				require.Contains(t, removed, ".tmp-legit.txt-1234")
				require.NotContains(t, removed, "legit.txt")
				require.FileExists(t, filepath.Join(basePath, "legit.txt"))
				require.NoFileExists(t, filepath.Join(basePath, "malicious.txt"))
				require.NoFileExists(t, filepath.Join(basePath, ".tmp-legit.txt-1234"))
			},
		},
		"Purge fails when the root was removed externally": {
			prepare: func(t *testing.T, parent string) string {
				t.Helper()
				if runtime.GOOS == "windows" {
					t.Skip("the custodian holds an open handle on the root, so Windows cannot remove it externally")
				}
				return parent
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				require.NoError(t, os.RemoveAll(c.BasePath()))
				_, err := c.Purge(func(string) bool { return false })
				require.Error(t, err, "purging a vanished root must error rather than report success")
				return nil
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
			},
		},
		"Purge on a nested custodian is scoped to its sub-tree": {
			makeSub: true,
			prepare: func(t *testing.T, parent string) string {
				t.Helper()
				return parent
			},
			preseed: func(t *testing.T, c *securefiles.Custodian) {
				t.Helper()
				require.NoError(t, c.WriteFile("keep.txt", []byte("keep")))
				require.NoError(t, c.WriteFile("remove.txt", []byte("remove")))
			},
			run: func(t *testing.T, c *securefiles.Custodian) []string {
				t.Helper()
				isAllowed := func(relPath string) bool { return relPath == "keep.txt" }
				removed, err := c.Purge(isAllowed)
				require.NoError(t, err)
				return removed
			},
			check: func(t *testing.T, c *securefiles.Custodian, removed []string) {
				t.Helper()
				basePath := c.BasePath()
				require.Equal(t, []string{"remove.txt"}, removed)
				require.FileExists(t, filepath.Join(basePath, "keep.txt"))
				require.NoFileExists(t, filepath.Join(basePath, "remove.txt"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			parent := t.TempDir()
			basePath := tc.prepare(t, parent)

			var c *securefiles.Custodian
			if tc.makeSub {
				root, err := securefiles.Open(basePath)
				require.NoError(t, err)
				t.Cleanup(func() { _ = root.Close() })
				c, err = root.Subdir("sub")
				require.NoError(t, err)
			} else {
				var err error
				c, err = securefiles.Open(basePath)
				require.NoError(t, err)
			}
			defer func() { _ = c.Close() }()

			if tc.preseed != nil {
				tc.preseed(t, c)
			}

			var removed []string
			if tc.run != nil {
				removed = tc.run(t, c)
			}

			tc.check(t, c, removed)
		})
	}
}
