// Internal regression tests for the custodian's write-and-publish pair: the bytes
// must travel through the descriptor the temporary was created with, and the publish
// must answer for that node only.

package securefiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWriteFilePublishesTheNodeItWrote pins the no-second-resolution promise. A
// tamperer that swaps the temporary's name in the window between its creation and
// the write — here by parking the fresh node and planting another stamped node at
// its name — must not redirect the write into the planted node, and must not have
// the custodian publish the planted node either.
func TestWriteFilePublishesTheNodeItWrote(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	require.NoError(t, err, "Setup: could not open the custodian")
	defer func() { _ = c.Close() }()

	require.NoError(t, c.WriteFile("victim.txt", []byte("victim")),
		"Setup: could not seed the stamped victim")

	betweenCreateAndWrite = func() {
		// A tamperer cannot predict the temporary's random suffix any more than this
		// test can: it watches the directory and swaps whatever fresh temporary the
		// target's name shows. Park the fresh temporary, then plant the victim at its
		// name: a name-based write would truncate and fill the victim, and a publish
		// that checks ownership alone would move the victim into the destination.
		entries, rerr := os.ReadDir(dir)
		require.NoError(t, rerr, "the swap must be able to list the sub-tree")
		var tmpName string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".tmp-target.txt-") {
				tmpName = e.Name()
				break
			}
		}
		require.NotEmpty(t, tmpName, "the swap must find the fresh temporary")
		require.NoError(t, os.Rename(filepath.Join(dir, tmpName), filepath.Join(dir, "parked.tmp")),
			"the swap must be able to move the fresh temporary")
		require.NoError(t, os.Link(filepath.Join(dir, "victim.txt"), filepath.Join(dir, tmpName)),
			"the swap must be able to plant the victim at the temporary's name")
	}
	defer func() { betweenCreateAndWrite = nil }()

	err = c.WriteFile("target.txt", []byte("attacker"))
	require.Error(t, err, "a publish whose name no longer holds the written node must refuse")
	require.ErrorIs(t, err, ErrNotOwned, "the refusal must carry the not-owned identity")

	got, err := os.ReadFile(filepath.Join(dir, "victim.txt"))
	require.NoError(t, err, "the victim node must still be readable under its own name")
	require.Equal(t, "victim", string(got),
		"the write must not have reached the node planted at the temporary's name")

	require.NoFileExists(t, filepath.Join(dir, "target.txt"),
		"a refused publish must not have replaced the destination")
}
