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
			// Every node answers as the custodian's own, so the ownership
			// check alone would let the publish through — the refusal under
			// test can only be the descriptor identity, which is the promise.
			readStampFn: func(*os.File) (stamp, bool, error) {
				return fileStamp(), true, nil
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	// The victim is planted stamped (the configured readStamp answers for
	// every node), so only the descriptor identity can refuse the publish.
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

// TestSubdirRefusesAReplacedRoot pins the identity tie Subdir rests on, the
// same tie Open performs for its own root: the nested root OpenRoot anchors
// on the node the heal verified, not a name resolved twice. A tamperer that
// swaps the node between the verification and the re-resolution — here by the
// child platform layer answering a different identity than the parent's for
// the same node — must leave the sub-tree refused, never a child custodian
// rooted at a node nobody verified. The child layer's identity seam is the
// same window an attacker has: one resolution answers the heal's node, the
// next answers whatever took its name.
func TestSubdirRefusesAReplacedRoot(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir: dir,
			// The heal's side of the tie: the parent answers the node the
			// verification opened, as the real layers do.
			createStampedFn: func(rel string, isDir bool) (*os.File, error) {
				require.True(t, isDir, "the heal creates a directory")
				require.NoError(t, os.Mkdir(filepath.Join(dir, rel), DirMode), "Setup: could not create the sub-tree")
				return nil, nil
			},
			// The tamper: the child layer answers the child-root's node as
			// another node entirely, which is what a swap under the name
			// looks like from the two ends of the tie.
			subFn: func() platformSys {
				return &fakeSys{
					dir: dir,
					identityFn: func(*os.File) (fileIdentity, bool) {
						return fileIdentity{volume: 1, file: [16]byte{1}}, true
					},
				}
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	sub, err := c.Subdir("sub")
	require.ErrorIs(t, err, ErrRootReplaced, "a root replaced between the two resolutions must be refused")
	require.Nil(t, sub, "a refused sub-tree must not yield a custodian")
}
