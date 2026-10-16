// Policy tests over a fake platformSys: every decision the custodian makes
// without knowing the mechanism — the refuse-at-open policy, the identity tie
// between the two resolutions, the ownership verdicts, the degraded-class
// fallback, the projection report — tested on any platform, with a fake whose
// every mechanism is configured by the case that needs it. The mechanisms
// themselves are pinned by the platform suites against real filesystems.

package securefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOpenRefusesARerootedTree pins the identity tie between the two
// resolutions of the tree root: the parent directory stays writable from
// inside an instance (ADR 2.01), so between the platform's establishment and
// the custodian's own open it can be made to point elsewhere, and the identity
// is what makes the mismatch a refused verification rather than a redirect to
// serve. A root identified once must keep identifying: a reopened root that
// cannot identify itself is a failed verification, not an absent one.
func TestOpenRefusesARerootedTree(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		established fileIdentity
		reopened    fileIdentity
		identified  bool
	}{
		"a tree swapped in between the resolutions": {
			established: fileIdentity{volume: 1},
			reopened:    fileIdentity{volume: 2},
			identified:  true,
		},
		"a root that identified once and stopped answering": {
			established: fileIdentity{volume: 1},
			identified:  false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := openFake(t, func(dir string) *fakeSys {
				return &fakeSys{
					dir: dir,
					establishRootFn: func() (fileIdentity, bool, error) {
						return tc.established, true, nil
					},
					identityFn: func(*os.File) (fileIdentity, bool) {
						return tc.reopened, tc.identified
					},
				}
			})
			require.Nil(t, c, "a refused construction must not hand out a custodian")

			require.ErrorIs(t, err, ErrRootReplaced,
				"a tree the identity does not vouch for must be refused")
		})
	}
}

// TestOpenAdoptsAnUnidentifiedRoot pins the concession rule's other side: a
// platform that never identified the root concedes once, at the first
// resolution, and serves — nothing was recorded, so there is nothing to
// compare, and refusing would take away the custodian from every filesystem
// without a usable identity.
func TestOpenAdoptsAnUnidentifiedRoot(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir: dir,
			establishRootFn: func() (fileIdentity, bool, error) {
				return fileIdentity{}, false, nil
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	require.NotNil(t, c, "an unidentified root is conceded once, not refused")
}

// TestRenameFallsBackToTheDegradedClass pins the fallback the Windows layer
// needs volumes to have: a POSIX rename the volume's class does not implement
// is retried with the degraded one, and a volume that implements it is never
// asked twice.
func TestRenameFallsBackToTheDegradedClass(t *testing.T) {
	t.Parallel()

	errClassUnsupported := errors.New("the volume does not implement the POSIX rename class")

	testCases := map[string]struct {
		firstCallErr error
	}{
		"a volume without the POSIX class falls back":     {firstCallErr: errClassUnsupported},
		"a volume with the POSIX class needs no fallback": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls []bool
			c, err := openFake(t, func(dir string) *fakeSys {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0600),
					"Setup: could not plant the node")
				return &fakeSys{
					dir: dir,
					// The planted node reads as the custodian's own, so the
					// ownership check passes and the case observes the class
					// fallback alone.
					readStampFn: func(*os.File) (stamp, bool, error) {
						return fileStamp(), true, nil
					},
					renameReplaceFn: func(_ *os.File, _, _ string, posix bool) error {
						calls = append(calls, posix)
						if len(calls) == 1 {
							return tc.firstCallErr
						}
						return nil
					},
					isRenameClassUnsupportedFn: func(err error) bool {
						return errors.Is(err, errClassUnsupported)
					},
				}
			})
			require.NoError(t, err, "Setup: could not open the custodian over the fake")
			t.Cleanup(func() { _ = c.Close() })

			err = c.Rename("f", "g")
			require.NoError(t, err, "the rename must succeed through the fallback")

			wantCalls := []bool{true, false}
			if tc.firstCallErr == nil {
				wantCalls = []bool{true}
			}
			require.Equal(t, wantCalls, calls,
				"the POSIX class is asked first, and the degraded one only when the volume lacks it")
		})
	}
}

// TestUnreadableStampSurfacesItsFailure pins what a stamp that cannot
// be read is: not an answer about the node. The ownership predicate surfaces
// the filesystem error rather than turning it into a verdict, and the rename
// refuses with the not-owned identity on top of it — the caller keeps treating
// the node as unverifiable rather than as owned.
func TestUnreadableStampSurfacesItsFailure(t *testing.T) {
	t.Parallel()

	queryFailure := errors.New("the stamp could not be read")

	testCases := map[string]struct {
		// run asks about the node "f" through one of the two verdicts.
		run func(c *Custodian) error
		// wantNotOwned asserts the verdict keeps the not-owned identity.
		wantNotOwned bool
	}{
		"the ownership predicate surfaces the failure": {
			run: func(c *Custodian) error { _, err := c.IsOwned("f"); return err },
		},
		"the rename keeps the not-owned identity and surfaces the failure": {
			run:          func(c *Custodian) error { return c.Rename("f", "g") },
			wantNotOwned: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := openFake(t, func(dir string) *fakeSys {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0600),
					"Setup: could not plant the node")
				return &fakeSys{
					dir: dir,
					readStampFn: func(*os.File) (stamp, bool, error) {
						return stamp{}, false, queryFailure
					},
				}
			})
			require.NoError(t, err, "Setup: could not open the custodian over the fake")
			t.Cleanup(func() { _ = c.Close() })

			err = tc.run(c)
			if tc.wantNotOwned {
				require.ErrorIs(t, err, ErrNotOwned, "an unreadable stamp is not an answer, so the node is not owned")
			}
			require.ErrorIs(t, err, queryFailure, "the filesystem failure must surface, not hide")
		})
	}
}

func TestRenameRefusesUnownedNodes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		stamp stamp
		found bool
	}{
		"a node carrying no stamp":  {found: false},
		"a node stamped by another": {stamp: stamp{uid: 1000, gid: 1000, mode: 0100600}, found: true},
		"a node stamped with the wrong mode": {
			stamp: stamp{uid: 0, gid: 0, mode: 0100644}, found: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := openFake(t, func(dir string) *fakeSys {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0600),
					"Setup: could not plant the node")
				return &fakeSys{
					dir: dir,
					readStampFn: func(*os.File) (stamp, bool, error) {
						return tc.stamp, tc.found, nil
					},
				}
			})
			require.NoError(t, err, "Setup: could not open the custodian over the fake")
			t.Cleanup(func() { _ = c.Close() })

			err = c.Rename("f", "g")
			require.ErrorIs(t, err, ErrNotOwned, "the rename should have been refused")
		})
	}
}

// TestCreateFileReportsAHeldNode pins the refusal's identity: a removal the
// platform classifies as a holder's refusal answers ErrNodeHeld — the verdict
// a caller waits out and retries — while the same removal without that
// classification stays the filesystem error it was, never the sentinel. The
// classification itself is the platform's; the Windows real-filesystem test
// pins it against a holder that shares no deletion.
func TestCreateFileReportsAHeldNode(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		held bool
	}{
		"a removal refused by a holder answers ErrNodeHeld":   {held: true},
		"a removal refused for another reason stays as it is": {held: false},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := openFake(t, func(dir string) *fakeSys {
				// A non-empty directory at the target name makes the removal
				// fail whatever privileges the test runs with; what the case
				// varies is only how the platform classifies that failure.
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "target", "occupied"), 0700),
					"Setup: could not plant the node to be removed")
				return &fakeSys{
					dir: dir,
					isHeldNodeFn: func(error) bool {
						return tc.held
					},
				}
			})
			require.NoError(t, err, "Setup: could not open the custodian over the fake")
			t.Cleanup(func() { _ = c.Close() })

			_, err = c.CreateFile("target")
			if tc.held {
				require.ErrorIs(t, err, ErrNodeHeld, "a holder's refusal must carry the held identity")
				return
			}
			require.Error(t, err, "the removal must still fail")
			require.NotErrorIs(t, err, ErrNodeHeld, "an unclassified refusal must not wear the held identity")
		})
	}
}

// TestIsOwnedAnswersNotOwnedForDirectories pins the predicate's one shape:
// a directory is never owned, whatever stands in it — the predicate accepts
// only the file stamp, and the stamp of a sub-tree root is for the
// projection, not for adoption.
func TestIsOwnedAnswersNotOwnedForDirectories(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{dir: dir}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	owned, err := c.IsOwned(".")
	require.NoError(t, err, "a directory is a verdict, not a failure")
	require.False(t, owned, "a directory is never owned")
}

// The cases below cover the construction and policy plumbing only a fake
// platform can fail: the fake's mechanism refuses where no real filesystem
// on a CI machine can, and each case also asserts the cleanup the refusal
// owes (release the hold, leave no node behind), which is behavior, not just
// coverage.

// TestOpenRefusesAnUnopenableRoot pins the refusal of a root the platform
// established but the custodian cannot open — here a plain file standing at
// the base path, the shape TestOpenErrors pins through the public Open — and
// that whatever the platform held before the refusal is released, which is
// the part only the fake can observe.
func TestOpenRefusesAnUnopenableRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	base := filepath.Join(tmp, "blocker")
	require.NoError(t, os.WriteFile(base, []byte("x"), 0600), "Setup: could not plant the blocking file")

	released := false
	c, err := open(base, func(string) platformSys {
		return &fakeSys{
			dir:       base,
			releaseFn: func() error { released = true; return nil },
		}
	})
	require.Error(t, err, "a base path the custodian cannot open as a root must be refused")
	require.Nil(t, c, "no custodian may be handed out for a refused root")
	require.True(t, released, "whatever the platform held before the refusal must be released")
}

// TestOpenRefusesWhatItCannotAttach pins the refusal of a root whose hold the
// platform cannot take, and that the refusal releases what was held.
func TestOpenRefusesWhatItCannotAttach(t *testing.T) {
	t.Parallel()

	attachFailure := errors.New("could not take the hold")
	released := false
	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir:       dir,
			attachFn:  func(*os.Root) error { return attachFailure },
			releaseFn: func() error { released = true; return nil },
		}
	})
	require.Nil(t, c, "a refused construction must not hand out a custodian")

	require.ErrorIs(t, err, attachFailure, "a root the platform cannot attach to must be refused")
	require.ErrorContains(t, err, "failed to attach", "the refusal must name the attach step")
	require.True(t, released, "the refusal must release what the platform held")
}

// TestSubdirRefusesWhatItCannotAttach pins the same refusal one level down: a
// sub-tree root whose hold cannot be taken is refused and the sub-custodian
// is not handed out, whatever the parent already serves.
func TestSubdirRefusesWhatItCannotAttach(t *testing.T) {
	t.Parallel()

	attachFailure := errors.New("could not take the sub-hold")
	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir: dir,
			createStampedFn: func(rel string, isDir bool) (*os.File, error) {
				require.True(t, isDir, "this case only creates the sub-tree root")
				return nil, os.Mkdir(filepath.Join(dir, rel), DirMode)
			},
			subFn: func() platformSys {
				return &fakeSys{attachFn: func(*os.Root) error { return attachFailure }}
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	sub, err := c.Subdir("sub")
	require.ErrorIs(t, err, attachFailure, "a sub-tree whose hold cannot be taken must be refused")
	require.Nil(t, sub, "no sub-custodian may be handed out for a refused sub-tree")
}

// TestWriteFileRefusesATemporaryItCannotWrite pins that a write whose
// temporary cannot take the bytes fails rather than publishing a partial or
// empty node.
func TestWriteFileRefusesATemporaryItCannotWrite(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ro"), []byte("x"), 0600),
			"Setup: could not plant the unwritable node")
		ro, err := os.Open(filepath.Join(dir, "ro"))
		require.NoError(t, err, "Setup: could not open the read-only node")
		t.Cleanup(func() { _ = ro.Close() })
		return &fakeSys{
			dir:             dir,
			createStampedFn: func(string, bool) (*os.File, error) { return ro, nil },
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	err = c.WriteFile("target.txt", []byte("data"))
	require.Error(t, err, "a write whose temporary cannot be written must fail")
	require.NoFileExists(t, filepath.Join(c.BasePath(), "target.txt"),
		"nothing may be published by a refused write")
}

// TestUnstatableNodesAreRefused pins that a node that cannot even answer a
// Stat is not a verdict about ownership and not a publishable source: the
// predicate surfaces the error, and the rename refuses rather than moving
// whatever the name might resolve to.
func TestUnstatableNodesAreRefused(t *testing.T) {
	t.Parallel()

	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir: dir,
			openNodeFn: func(string, openProfile) (*os.File, error) {
				f, err := os.Open(dir)
				require.NoError(t, err, "Setup: could not open a node to close")
				require.NoError(t, f.Close(), "Setup: could not close the node")
				return f, nil
			},
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	_, err = c.IsOwned("f")
	require.Error(t, err, "a node that cannot answer a Stat is not a verdict about ownership")

	err = c.Rename("f", "g")
	require.Error(t, err, "a source that cannot answer a Stat must not be published")
}

// TestCheckProjectionNamesTheResolvedLocation pins the report's full shape: a
// root whose final path differs from the name it was handed is reported by
// where it really is, so the report cannot read as a contradiction.
func TestCheckProjectionNamesTheResolvedLocation(t *testing.T) {
	t.Parallel()

	const redirected = `\\server\share\publicdir`
	c, err := openFake(t, func(dir string) *fakeSys {
		return &fakeSys{
			dir:         dir,
			finalPathFn: func() string { return redirected },
		}
	})
	require.NoError(t, err, "Setup: could not open the custodian over the fake")
	t.Cleanup(func() { _ = c.Close() })

	err = c.CheckProjection()
	require.ErrorIs(t, err, ErrRemoteVolume, "a root the filesystem resolves into a share must be reported")
	require.ErrorContains(t, err, "resolving to", "the report must name the resolved location")
	require.ErrorContains(t, err, redirected, "the report must carry where the root really is")
}

// openFake opens a custodian whose OS layer is the fake, configured by the
// case before the custodian is built.
func openFake(t *testing.T, configure func(dir string) *fakeSys) (*Custodian, error) {
	t.Helper()
	dir := t.TempDir()
	sys := configure(dir)
	return open(dir, func(string) platformSys { return sys })
}

// fakeSys is a platformSys whose every mechanism is a configurable stand-in,
// so the custodian's policy can be exercised without the real filesystem
// being able to produce the condition under test.
//
// The defaults are split by when they run. The construction-time mechanisms
// (establishRoot, identity, sub, attach, release) answer what open() needs to
// hand out a custodian — a root established, an identity that matches, no
// hold to take — because every case needs that plumbing and no verdict rests
// on it. Every other mechanism the case does not configure answers a panic
// naming the method, so a forgotten wire fails the test loudly rather than
// silently. openNode is the one exception with a working default: most cases
// need a real node opened, and the default honours the contract's rule for it
// — a link standing at the name is refused, never followed — so a case riding
// the default cannot pass a policy that follows one.
type fakeSys struct {
	// dir is the tree the fake stands in for, so a case can answer with real
	// files where the policy reads through the node it was handed.
	dir string

	establishRootFn            func() (fileIdentity, bool, error)
	subFn                      func() platformSys
	attachFn                   func(*os.Root) error
	releaseFn                  func() error
	identityFn                 func(*os.File) (fileIdentity, bool)
	createStampedFn            func(rel string, dir bool) (*os.File, error)
	stampDirFn                 func(rel string) error
	openNodeFn                 func(rel string, profile openProfile) (*os.File, error)
	readStampFn                func(*os.File) (stamp, bool, error)
	renameReplaceFn            func(f *os.File, oldRel, newRel string, posix bool) error
	isRenameClassUnsupportedFn func(err error) bool
	isHeldNodeFn               func(err error) bool
	isLinkFn                   func(rel string) (bool, error)
	finalPathFn                func() string
	remoteVolumeFn             func(string) (bool, volumeKind)
}

// establishRoot defaults to an established root, so open() can hand out a
// custodian: the real tree at dir stands in for the one the default claims to
// have created. See the fakeSys doc for the split between defaults and panics.
func (s *fakeSys) establishRoot() (fileIdentity, bool, error) {
	if s.establishRootFn != nil {
		return s.establishRootFn()
	}
	return fileIdentity{}, true, nil
}

func (s *fakeSys) sub() platformSys {
	if s.subFn != nil {
		return s.subFn()
	}
	return &fakeSys{dir: s.dir}
}

func (s *fakeSys) attach(root *os.Root) error {
	if s.attachFn != nil {
		return s.attachFn(root)
	}
	return nil
}

func (s *fakeSys) release() error {
	if s.releaseFn != nil {
		return s.releaseFn()
	}
	return nil
}

func (s *fakeSys) identity(f *os.File) (fileIdentity, bool) {
	if s.identityFn != nil {
		return s.identityFn(f)
	}
	return fileIdentity{}, true
}

func (s *fakeSys) createStamped(rel string, dir bool) (*os.File, error) {
	if s.createStampedFn != nil {
		return s.createStampedFn(rel, dir)
	}
	panic("fakeSys.createStamped is not configured by the case")
}

func (s *fakeSys) stampDir(rel string) error {
	if s.stampDirFn != nil {
		return s.stampDirFn(rel)
	}
	panic("fakeSys.stampDir is not configured by the case")
}

func (s *fakeSys) openNode(rel string, profile openProfile) (*os.File, error) {
	if s.openNodeFn != nil {
		return s.openNodeFn(rel, profile)
	}
	// The default is the contract, not a shortcut: a link standing at the name
	// answers ErrPathEscapes, never its target. The lstat-then-open pair is
	// good enough for a stand-in; the real platforms refuse in the open itself.
	fi, err := os.Lstat(filepath.Join(s.dir, rel))
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, ErrPathEscapes
	}
	return os.Open(filepath.Join(s.dir, rel))
}

func (s *fakeSys) readStamp(f *os.File) (stamp, bool, error) {
	if s.readStampFn != nil {
		return s.readStampFn(f)
	}
	panic("fakeSys.readStamp is not configured by the case")
}

func (s *fakeSys) renameReplace(f *os.File, oldRel, newRel string, posix bool) error {
	if s.renameReplaceFn != nil {
		return s.renameReplaceFn(f, oldRel, newRel, posix)
	}
	panic("fakeSys.renameReplace is not configured by the case")
}

func (s *fakeSys) isRenameClassUnsupported(err error) bool {
	if s.isRenameClassUnsupportedFn != nil {
		return s.isRenameClassUnsupportedFn(err)
	}
	return false
}

func (s *fakeSys) isHeldNode(err error) bool {
	if s.isHeldNodeFn != nil {
		return s.isHeldNodeFn(err)
	}
	return false
}

func (s *fakeSys) isLink(rel string) (bool, error) {
	if s.isLinkFn != nil {
		return s.isLinkFn(rel)
	}
	panic("fakeSys.isLink is not configured by the case")
}

func (s *fakeSys) finalPath() string {
	if s.finalPathFn != nil {
		return s.finalPathFn()
	}
	return s.dir
}

// remoteVolume answers the lexical verdict the contract fixes: a UNC prefix
// is remote whatever the platform, and nothing else is answerable without
// platform state the fake does not carry.
func (s *fakeSys) remoteVolume(path string) (bool, volumeKind) {
	if s.remoteVolumeFn != nil {
		return s.remoteVolumeFn(path)
	}
	return lexicalRemoteVolume(path)
}
