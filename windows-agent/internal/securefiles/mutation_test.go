// Cross-platform mutation semantics: atomic writes leave no temporaries and
// tolerate leftover ones, and Remove/Rename/RemoveAll obey containment. The
// Windows-specific sharing-semantics race lives in mutation_windows_test.go.

package securefiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
)

func TestMutations(t *testing.T) {
	t.Parallel()

	testCases := map[string]func(t *testing.T, c *securefiles.Custodian, dir string){
		"quick successive writes leave no leftover temp files":    testQuickSuccessiveWritesNoLeftoverTemp,
		"write succeeds when a leftover temp file exists":         testWriteSucceedsWithLeftoverTemp,
		"remove, rename, remove-all work and reject path escapes": testRemoveRemoveAllAndRename,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c, err := securefiles.Open(dir)
			require.NoError(t, err)
			defer func() { _ = c.Close() }()

			tc(t, c, dir)
		})
	}
}

func testQuickSuccessiveWritesNoLeftoverTemp(t *testing.T, c *securefiles.Custodian, dir string) {
	t.Helper()
	for range 50 {
		err := c.WriteFile("rapid.txt", []byte("version"))
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), ".tmp-"), "Found leftover temp file: %s", entry.Name())
	}
}

func testWriteSucceedsWithLeftoverTemp(t *testing.T, c *securefiles.Custodian, dir string) {
	t.Helper()
	leftoverTemp := filepath.Join(dir, ".tmp-target.txt-12345678")
	err := os.WriteFile(leftoverTemp, []byte("leftover"), 0600)
	require.NoError(t, err)

	err = c.WriteFile("target.txt", []byte("fresh content"))
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, err)
	require.Equal(t, "fresh content", string(content))
}

func testRemoveRemoveAllAndRename(t *testing.T, c *securefiles.Custodian, dir string) {
	t.Helper()
	err := c.WriteFile("f1.txt", []byte("f1"))
	require.NoError(t, err)

	err = c.Rename("f1.txt", "f2.txt")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "f1.txt"))
	require.True(t, os.IsNotExist(err))

	f2Content, err := os.ReadFile(filepath.Join(dir, "f2.txt"))
	require.NoError(t, err)
	require.Equal(t, "f1", string(f2Content))

	err = c.Remove("f2.txt")
	require.NoError(t, err)

	subC, err := c.Subdir("tree")
	require.NoError(t, err)
	err = subC.WriteFile("nested.txt", []byte("nested"))
	require.NoError(t, err)

	// The nested custodian holds an open handle on "tree": on Windows the sub-tree
	// cannot be removed until that handle is closed, so close it first to keep
	// the test portable.
	require.NoError(t, subC.Close())

	err = c.RemoveAll("tree")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "tree"))
	require.True(t, os.IsNotExist(err))

	escapes := []string{"../escape.txt", "..\\escape.txt", "/etc/passwd"}
	for _, esc := range escapes {
		require.ErrorIs(t, c.Remove(esc), securefiles.ErrPathEscapes)
		require.ErrorIs(t, c.RemoveAll(esc), securefiles.ErrPathEscapes)
		require.ErrorIs(t, c.Rename(esc, "valid.txt"), securefiles.ErrPathEscapes)
		require.ErrorIs(t, c.Rename("valid.txt", esc), securefiles.ErrPathEscapes)
	}
}
