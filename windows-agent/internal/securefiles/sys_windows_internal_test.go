//go:build windows

// Internal tests over the Windows platform layer: they drive the NT syscall seams to
// reproduce filesystems this machine does not have, and pin ADR 2.02 throughout — a
// refusal degrades the custodian loudly and never fails closed.

package securefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestOpenStampingFailureDegrades pins ADR 2.02 at construction: a filesystem that
// refuses the attribute write must leave a degraded custodian that still works, reported
// loudly, never a failure to start. Both roots reach the same stamping call: one adopted
// from a pre-existing directory, one created by ensureRoot itself. A refusal on a capable
// filesystem is not degradation and must reach the caller instead.
func TestOpenStampingFailureDegrades(t *testing.T) {
	testCases := map[string]struct {
		// preCreate leaves the root on disk before Open, so it is adopted rather than created.
		preCreate bool
		// failStatus makes the attribute write fail with this NT status.
		failStatus windows.NTStatus

		wantOpenErr  bool
		wantDegraded bool
	}{
		"a pre-existing root on a filesystem without attribute support": {
			preCreate:    true,
			failStatus:   windows.STATUS_EAS_NOT_SUPPORTED,
			wantDegraded: true,
		},
		// The second run on a volume that cannot carry the watermark. The first created
		// the root and degraded; now the root is there, so it is adopted, and adoption is
		// the only path that asks to set the attribute on a node already in place. That
		// call answers STATUS_INVALID_DEVICE_REQUEST rather than the status the creating
		// call gives, so a set missing it refuses to start on the second run of exactly
		// the filesystem ADR 2.02 exists to keep serving.
		"a pre-existing root on a filesystem that cannot store the attribute": {
			preCreate:    true,
			failStatus:   windows.STATUS_INVALID_DEVICE_REQUEST,
			wantDegraded: true,
		},
		// Our own call being wrong must not read as the machine being incapable.
		"a pre-existing root whose stamp buffer is rejected": {
			preCreate:   true,
			failStatus:  windows.STATUS_INVALID_PARAMETER,
			wantOpenErr: true,
		},
		// The root exists and can hold attributes, but the write was refused. Adopting it
		// as "degraded" would hand the distro an unowned sub-tree and call that expected.
		"a pre-existing root whose stamp is refused": {
			preCreate:   true,
			failStatus:  windows.STATUS_ACCESS_DENIED,
			wantOpenErr: true,
		},
		"a root created by ensureRoot itself": {failStatus: windows.STATUS_ACCESS_DENIED},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			rootDir := filepath.Join(t.TempDir(), ".cloud-init")
			if tc.preCreate {
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")
			}

			hook := test.NewGlobal()
			defer hook.Reset()

			cust, err := OpenRefusingStamp(rootDir, uint32(tc.failStatus))
			if tc.wantOpenErr {
				require.Error(t, err, "a refused stamp on a capable filesystem must reach the caller")
				return
			}
			require.NoError(t, err, "an unsupported filesystem must never fail closed")
			defer cust.Close()

			// A root ensureRoot creates carries its stamp in the NtCreateFile call itself
			// (ADR 2.01), so it never reaches the attribute write this seam refuses. Only
			// an adopted root is stamped separately, and only it can degrade here.
			require.Equal(t, tc.wantDegraded, cust.IsDegraded(), "unexpected degraded state")

			// Nothing is written anywhere: the agent has no log yet, so the finding is
			// held in the custodian until the caller asks for it.
			require.Empty(t, hook.AllEntries(), "the custodian must not log; it reports through CheckProjection")

			if tc.wantDegraded {
				require.ErrorIs(t, cust.CheckProjection(), ErrDegraded, "CheckProjection must name the degradation")
				require.Contains(t, cust.CheckProjection().Error(), "could not stamp the pre-existing root",
					"CheckProjection must carry what revealed it")
				return
			}
			require.NoError(t, cust.CheckProjection(), "a stamped root has nothing to report")
		})
	}
}

// TestCreateNode covers what createNode does when the filesystem will not take the stamp.
// ADR 2.02 requires it to fail open: an EA rejection degrades the custodian and falls back
// to plain creation, while an unrelated failure is a real error. Once degraded it stops
// reaching NtCreateFile at all, and must still create nodes and still refuse a collision
// rather than silently adopting whatever is already there.
func TestCreateNode(t *testing.T) {
	testCases := map[string]struct {
		// failCreation makes NtCreateFile fail with this status; zero leaves it alone.
		failCreation uint32
		// preDegraded marks the filesystem as unable to carry the stamp before the call.
		preDegraded bool
		// seedExisting puts a node at the path before the call.
		seedExisting bool
		isDir        bool

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
		// A wrong call of ours must stay loud rather than be absorbed as a property of
		// the filesystem, which would fail open on every machine at once.
		"a rejected buffer surfaces as an error": {
			failCreation: uint32(windows.STATUS_INVALID_PARAMETER),
			wantErr:      true,
		},
		"a degraded custodian still creates files": {
			preDegraded: true,
			wantExists:  true, wantDegraded: true,
		},
		"a degraded custodian still creates directories": {
			preDegraded: true, isDir: true,
			wantExists: true, wantDegraded: true,
		},
		"an existing node is refused, not adopted": {
			preDegraded: true, seedExisting: true,
			wantErr: true, wantExists: true, wantDegraded: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			const node = "node"
			if tc.seedExisting {
				require.NoError(t, cust.sys.createNode(node, tc.isDir), "Setup: could not seed the existing node")
			}
			if tc.preDegraded {
				cust.SetDegraded(true)
			}
			if tc.failCreation != 0 {
				cust.FailCreation(tc.failCreation)
			}

			err = cust.sys.createNode(node, tc.isDir)
			if tc.wantErr {
				require.Error(t, err, "the creation should have been refused")
			} else {
				require.NoError(t, err, "the creation should have succeeded")
			}

			path := filepath.Join(dir, node)
			if tc.wantExists {
				require.DirExists(t, filepath.Dir(path), "Setup: the sub-tree should exist")
				_, statErr := os.Stat(path)
				require.NoError(t, statErr, "the node should exist on disk")
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
		// such as a network redirector. It describes the root as first resolved.
		unverifiable bool
		// loseIdentity models the reopened node failing to identify itself although the
		// original did, as a swap onto a filesystem that exposes no identity would.
		loseIdentity bool

		wantErr error
	}{
		"the directory that was created":      {},
		"a directory swapped in behind it":    {replaced: true, wantErr: ErrRootReplaced},
		"no identity to compare against":      {unverifiable: true},
		"swapped, but no identity to compare": {replaced: true, unverifiable: true},

		// A root that identified itself once must keep doing so. Accepting a reopened
		// handle that cannot be identified would let a swap onto a filesystem without
		// identities walk straight past the comparison.
		"identity lost between the two resolutions": {loseIdentity: true, wantErr: ErrRootReplaced},
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
			if tc.loseIdentity {
				sys.nt.identity = func(windows.Handle) (fileIdentity, bool) { return fileIdentity{}, false }
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

// TestNTPathEncodingIsRefused pins that a relative path the NT layer cannot encode is
// refused rather than silently truncated at the NUL. NT counted strings carry an explicit
// length, so a truncating implementation would let a caller name "good\x00evil" and have
// the kernel act on "good" instead. Every case therefore seeds a real node called "good":
// if the NUL were dropped, the call would succeed against that node instead of failing.
func TestNTPathEncodingIsRefused(t *testing.T) {
	t.Parallel()

	const bad = "good\x00evil"

	testCases := map[string]struct {
		seedFile bool
		seedDir  bool

		call  func(s *platformSys) error
		after func(t *testing.T, dir string)
	}{
		"ownership check": {
			seedFile: true,
			call: func(s *platformSys) error {
				owned, err := s.isOwned(bad)
				if err == nil && owned {
					return nil // truncated onto the seeded node
				}
				return err
			},
		},
		"node creation": {
			call: func(s *platformSys) error { return s.createNode(bad, false) },
			after: func(t *testing.T, dir string) {
				t.Helper()
				require.NoFileExists(t, filepath.Join(dir, "good"), "a truncated path must not create the shorter node")
			},
		},
		"directory open": {
			seedDir: true,
			call: func(s *platformSys) error {
				h, err := s.openDirNoReparse(bad)
				if err == nil {
					closeHandle(h)
				}
				return err
			},
		},
		"rename source": {
			seedFile: true,
			call:     func(s *platformSys) error { return s.renameNode(bad, "moved.txt") },
			after: func(t *testing.T, dir string) {
				t.Helper()
				require.FileExists(t, filepath.Join(dir, "good"), "a truncated path must not move the shorter node")
			},
		},
		"sub-directory stamp": {
			seedDir: true,
			call:    func(s *platformSys) error { return s.stampSubdir(bad) },
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			if tc.seedFile {
				require.NoError(t, cust.WriteFile("good", []byte("seed")), "Setup: could not seed the file")
			}
			if tc.seedDir {
				require.NoError(t, os.Mkdir(filepath.Join(dir, "good"), DirMode), "Setup: could not seed the directory")
			}

			require.Error(t, tc.call(cust.sys), "an unencodable path must be refused, never truncated")

			if tc.after != nil {
				tc.after(t, dir)
			}
		})
	}
}

// TestRenameNode covers the two conditions renameNode has to survive on its own: a
// destination leaf it cannot encode, which is checked only after the source is open, and
// a filesystem with no FILE_RENAME_INFORMATION_EX at all, where the rename must still
// happen through the original information class rather than be reported as a failure.
func TestRenameNode(t *testing.T) {
	testCases := map[string]struct {
		to string
		// withoutRenameInfoEx models a filesystem that rejects the newer information class.
		withoutRenameInfoEx bool

		wantErr bool
	}{
		"an unencodable destination is refused":                 {to: "bad\x00name", wantErr: true},
		"a filesystem without the newer info class still moves": {to: "dst.txt", withoutRenameInfoEx: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			require.NoError(t, cust.WriteFile("src.txt", []byte("payload")), "Setup: could not seed the source")
			if tc.withoutRenameInfoEx {
				cust.WithoutRenameInfoEx()
			}

			err = cust.sys.renameNode("src.txt", tc.to)
			if tc.wantErr {
				require.Error(t, err, "the rename should have been refused")
				require.FileExists(t, filepath.Join(dir, "src.txt"), "the source must survive a refused rename")
				return
			}

			require.NoError(t, err, "the rename should have succeeded")
			require.NoFileExists(t, filepath.Join(dir, "src.txt"), "the source must be gone after the rename")
			got, err := os.ReadFile(filepath.Join(dir, tc.to))
			require.NoError(t, err, "the destination must exist after the rename")
			require.Equal(t, "payload", string(got), "the rename must move the content")
		})
	}
}

// TestIdentityOf pins the best-effort contract of the identity check: a handle it cannot
// interrogate yields "unknown" rather than a zero identity, which would compare equal to
// another unknown one and wrongly certify a swapped root.
func TestIdentityOf(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// realDir interrogates a handle to an actual directory instead of an invalid one.
		realDir bool

		wantKnown bool
	}{
		"an uninterrogable handle": {},
		"an open directory":        {realDir: true, wantKnown: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// ensureRoot closes its handle, so a live one has to come from elsewhere.
			h := windows.InvalidHandle
			if tc.realDir {
				f, err := os.Open(t.TempDir())
				require.NoError(t, err, "Setup: could not open the directory")
				defer func() { _ = f.Close() }()
				h = windows.Handle(f.Fd())
			}

			_, ok := identityOf(h)
			require.Equal(t, tc.wantKnown, ok, "unexpected identity availability")
		})
	}
}

// TestStampSubdirDegrades covers the two ways stampSubdir declines to stamp: it is a
// no-op once the filesystem is known not to carry the attribute, and a refusal of the
// attribute write degrades the custodian instead of failing the call, per ADR 2.02 -
// but only when the filesystem cannot carry attributes at all. A write refused on a
// capable filesystem is an anomaly, and is reported rather than recorded as degradation.
func TestStampSubdirDegrades(t *testing.T) {
	testCases := map[string]struct {
		alreadyDegraded bool
		// failStatus makes the attribute write fail with this NT status.
		failStatus windows.NTStatus

		wantErr      bool
		wantDegraded bool
		// wantCause is what CheckProjection must relay about the transition. A custodian
		// already degraded before the call keeps the cause it had: the first one explains
		// the rest, and later failures are its consequences.
		wantCause string
	}{
		"a degraded filesystem is not stamped again": {
			alreadyDegraded: true,
			wantDegraded:    true,
			wantCause:       ErrForcedDegradation.Error(),
		},
		"a filesystem without attribute support degrades": {
			failStatus:   windows.STATUS_EAS_NOT_SUPPORTED,
			wantDegraded: true,
			wantCause:    "could not stamp the sub-directory sub",
		},
		// The answer a volume without attribute support actually gives when asked to set
		// one on a node that is already there. It is the only answer this path can see on
		// such a volume, because a sub-tree left by an earlier run is adopted, not created.
		"a filesystem that cannot store the attribute degrades": {
			failStatus:   windows.STATUS_INVALID_DEVICE_REQUEST,
			wantDegraded: true,
			wantCause:    "could not stamp the sub-directory sub",
		},
		// Something stopped us securing a filesystem that is perfectly able to be
		// secured - a filter driver, an ACL. Calling that "degraded" would tell the
		// distro to expect unowned nodes forever, on evidence of one refused write.
		"a refused write on a capable filesystem is an error, not degradation": {
			failStatus: windows.STATUS_ACCESS_DENIED,
			wantErr:    true,
		},
		// The generic answer to a call whose arguments are wrong. No volume without
		// attribute support returns it, but our own buffer or flags would if they ever
		// became wrong, and absorbing that as degradation would fail open everywhere at
		// once rather than say so.
		"a rejected buffer is an error, not degradation": {
			failStatus: windows.STATUS_INVALID_PARAMETER,
			wantErr:    true,
		},
		"an inconsistent attribute list is an error, not degradation": {
			failStatus: windows.STATUS_EA_LIST_INCONSISTENT,
			wantErr:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), DirMode), "Setup: could not pre-create the sub-directory")

			if tc.alreadyDegraded {
				cust.SetDegraded(true)
			}
			if tc.failStatus != 0 {
				cust.FailStamping(uint32(tc.failStatus))
			}

			err = cust.sys.stampSubdir("sub")
			if tc.wantErr {
				require.Error(t, err, "a refusal must be reported, not swallowed")
			} else {
				require.NoError(t, err, "an unsupported filesystem must never fail closed")
			}

			require.Equal(t, tc.wantDegraded, cust.IsDegraded(), "unexpected degraded state")
			if tc.wantCause == "" {
				require.NoError(t, cust.degradationCause(), "nothing new should have been recorded")
			} else {
				require.ErrorContains(t, cust.degradationCause(), tc.wantCause, "unexpected recorded cause")
			}
		})
	}
}

// TestEnsureRootRejectsUnusableBasePath pins that a base path the platform cannot
// establish is reported at construction, where it can still be acted on.
func TestEnsureRootRejectsUnusableBasePath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), FileMode), "Setup: could not seed the blocking file")

	testCases := map[string]struct {
		basePath string
	}{
		"a parent that is a file, not a directory": {basePath: filepath.Join(blocker, "child", "root")},
		"a leaf name the NT layer cannot encode":   {basePath: filepath.Join(tmp, "bad\x00name")},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := newPlatformSys(tc.basePath)
			require.Error(t, err, "an unusable base path must be refused")
		})
	}
}

// TestCheckProjection pins what the custodian reports about its own sub-tree, and that a
// healthy one reports nothing. A volume this machine does not own stores the stamp
// faithfully, so a custodian that looks healthy from Windows is not evidence that
// instances see a root-owned sub-tree; ADR 2.02 requires the condition to be named rather
// than refused.
func TestCheckProjection(t *testing.T) {
	testCases := map[string]struct {
		basePath string
		degraded bool
		cause    string

		// wantErrs lists every condition CheckProjection must name. Empty means a
		// healthy sub-tree, for which it says nothing at all.
		wantErrs []error
		wantText string
	}{
		"a healthy sub-tree on a local drive": {basePath: `C:\Users\someone\.ubuntupro`},
		"a sub-tree on a UNC path": {
			basePath: `\\server\share\publicdir`,
			wantErrs: []error{ErrRemoteVolume},
			wantText: "a UNC path",
		},
		// What GetFinalPathNameByHandle hands back for a directory reached through a link
		// into a share: the name the caller used says "C:", so only the resolved form
		// reveals that the sub-tree is not on this machine.
		"a sub-tree resolved to a redirected share": {
			basePath: `\\?\UNC\server\share\publicdir`,
			wantErrs: []error{ErrRemoteVolume},
			wantText: "a UNC path",
		},
		"a sub-tree resolved to a local drive": {
			basePath: `\\?\C:\Users\someone\.ubuntupro`,
		},
		"a sub-tree that cannot be stamped": {
			basePath: `C:\Users\someone\.ubuntupro`,
			degraded: true,
			cause:    "could not stamp the pre-existing root: access denied",
			wantErrs: []error{ErrDegraded},
			wantText: "access denied",
		},
		// Both conditions hold at once and neither hides the other: an operator reading
		// one line must learn everything that is wrong with the sub-tree.
		"a sub-tree that is both remote and unstampable": {
			basePath: `\\server\share\publicdir`,
			degraded: true,
			wantErrs: []error{ErrDegraded, ErrRemoteVolume},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &Custodian{basePath: tc.basePath}
			if tc.degraded {
				cause := tc.cause
				if cause == "" {
					cause = "the filesystem cannot carry extended attributes"
				}
				deg := &degradation{}
				deg.note(errors.New(cause))
				c.sys = &platformSys{deg: deg}
			}

			gaps := c.CheckProjection()
			if len(tc.wantErrs) == 0 {
				require.NoError(t, gaps, "a healthy sub-tree is not a condition worth reporting")
				return
			}

			for _, want := range tc.wantErrs {
				require.ErrorIs(t, gaps, want, "CheckProjection must name every condition that applies")
			}
			if tc.wantText != "" {
				require.Contains(t, gaps.Error(), tc.wantText, "CheckProjection must carry what was found")
			}
		})
	}
}

// TestEnsureRootRefusesAnUnstampableRoot pins the one condition ADR 2.02 lets the
// custodian adopt an unstamped root for: a filesystem that cannot carry extended
// attributes at all. Everything else that can refuse the creation — an ACL that denies
// the access the stamp needs, a filter driver, a sharing violation — leaves a directory
// that could be stamped, and adopting it would serve an unstamped root to every instance
// while reporting nothing wrong.
func TestEnsureRootRefusesAnUnstampableRoot(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// failStatus makes the root creation fail with this NT status.
		failStatus windows.NTStatus
		// preCreate leaves the directory on disk, so the fallback would find it in place
		// and succeed, which is what made the refusal invisible.
		preCreate bool

		wantErr      bool
		wantDegraded bool
	}{
		"a filesystem without attribute support degrades": {
			failStatus:   windows.STATUS_EAS_NOT_SUPPORTED,
			wantDegraded: true,
		},
		"an unsupported operation degrades": {
			failStatus:   windows.STATUS_NOT_SUPPORTED,
			wantDegraded: true,
		},
		"a refused creation reaches the caller": {
			failStatus: windows.STATUS_ACCESS_DENIED,
			wantErr:    true,
		},
		// No volume without attribute support answers this; our own buffer or flags would.
		"a rejected buffer reaches the caller": {
			failStatus: windows.STATUS_INVALID_PARAMETER,
			wantErr:    true,
		},
		"a refused creation over an existing directory reaches the caller": {
			failStatus: windows.STATUS_ACCESS_DENIED,
			preCreate:  true,
			wantErr:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := filepath.Join(t.TempDir(), ".ubuntupro")
			if tc.preCreate {
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")
			}

			cust, err := OpenRefusingCreation(rootDir, uint32(tc.failStatus))
			if tc.wantErr {
				require.Error(t, err, "a root that could carry the stamp must not be adopted without one")
				require.Nil(t, cust, "no custodian may be handed out for a refused root")
				return
			}
			require.NoError(t, err, "an unstampable filesystem must never fail closed")
			defer cust.Close()

			require.Equal(t, tc.wantDegraded, cust.IsDegraded(), "unexpected degraded state")
			require.ErrorIs(t, cust.CheckProjection(), ErrDegraded, "the gap must be reported")
		})
	}
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
