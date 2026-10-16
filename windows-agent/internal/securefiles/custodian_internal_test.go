// Internal regression tests for the custodian's write-and-publish pair: the bytes
// must travel through the descriptor the temporary was created with, and the publish
// must answer for that node only.

package securefiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWriteFilePublishesTheNodeItWrote pins the no-second-resolution promise.
// A tamperer that swaps the temporary's name in the window between its
// creation and the write — here by parking the fresh node and planting another
// stamped node at its name — must not redirect the write into the planted
// node, and must not have the custodian publish the planted node either. The
// swap rides the fake platform layer's creation, which is the same window an
// attacker has: the moment the node exists and before the writer fills it.
func TestWriteFilePublishesTheNodeItWrote(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir: dir,
			createStampedFn: func(rel string, isDir bool) (*os.File, error) {
				require.False(t, isDir, "this case writes a file")

				require.NoError(t, os.WriteFile(filepath.Join(dir, rel), nil, FileMode),
					"the swap must be able to create the fresh temporary")

				// Park the fresh node, then plant the victim at its name: a
				// name-based write would truncate and fill the victim, and a
				// publish that checks ownership alone would move the victim
				// into the destination. The park goes through a name no handle
				// is open on, as an attacker's would: the real creation shares
				// deletion with the writer, but the parked node is reopened
				// below and answers for the same node either way.
				require.NoError(t, os.Rename(filepath.Join(dir, rel), filepath.Join(dir, "parked.tmp")),
					"the swap must be able to park the fresh temporary")
				require.NoError(t, os.Link(filepath.Join(dir, "victim.txt"), filepath.Join(dir, rel)),
					"the swap must be able to plant the victim")

				fresh, err := os.OpenFile(filepath.Join(dir, "parked.tmp"), os.O_WRONLY, FileMode)
				require.NoError(t, err, "the swap must be able to reach the parked node")
				return fresh, nil
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	// The victim is planted stamped: the fake answers for the stamp, so the
	// ownership check alone would let the publish through — the refusal under
	// test can only be the descriptor identity, which is the promise.
	require.NoError(t, os.WriteFile(filepath.Join(c.BasePath(), "victim.txt"), []byte("victim"), 0600),
		"Setup: could not plant the victim")

	err = c.WriteFile("target.txt", []byte("attacker"))
	require.Error(t, err, "a publish whose name no longer holds the written node must refuse")
	require.ErrorIs(t, err, ErrNotOwned, "the refusal must carry the not-owned identity")

	got, err := os.ReadFile(filepath.Join(c.BasePath(), "victim.txt"))
	require.NoError(t, err, "the victim node must still be readable under its own name")
	require.Equal(t, "victim", string(got),
		"the write must not have reached the node planted at the temporary's name")

	require.NoFileExists(t, filepath.Join(c.BasePath(), "target.txt"),
		"a refused publish must not have replaced the destination")
}
