//go:build windows

// Internal tests over the Windows platform layer: they drive the NT syscall seams to
// reproduce filesystems this machine does not have, and pin ADR 2.02 throughout — a
// sub-tree that cannot be stamped is refused rather than served unstamped.

package securefiles

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestCreateNode covers what createNode does when the filesystem will not take the stamp.
// ADR 2.02 requires it to refuse: a node created without the stamp is a node no instance
// sees as root-owned. A collision is refused rather than silently adopted, whatever the
// filesystem answered.
func TestCreateNode(t *testing.T) {
	testCases := map[string]struct {
		// failCreation makes NtCreateFile fail with this status; zero leaves it alone.
		failCreation uint32
		// seedExisting puts a node at the path before the call.
		seedExisting bool
		isDir        bool

		wantErr    bool
		wantExists bool
	}{
		// A filesystem that cannot carry the watermark is refused rather than served: the
		// node would be created, but no instance would see it as root-owned.
		"a filesystem without attribute support is refused": {
			failCreation: uint32(windows.STATUS_EAS_NOT_SUPPORTED),
			wantErr:      true,
		},
		"a filesystem that cannot store the attribute is refused": {
			failCreation: uint32(windows.STATUS_INVALID_DEVICE_REQUEST),
			wantErr:      true,
		},
		"an unrelated failure surfaces as an error": {
			failCreation: uint32(windows.STATUS_ACCESS_DENIED),
			wantErr:      true,
		},
		// A wrong call of ours must stay loud rather than be read as a property of the
		// filesystem, which would blame the machine for our mistake.
		"a rejected buffer surfaces as an error": {
			failCreation: uint32(windows.STATUS_INVALID_PARAMETER),
			wantErr:      true,
		},
		"a file is created and stamped":      {wantExists: true},
		"a directory is created and stamped": {isDir: true, wantExists: true},
		"an existing node is refused, not adopted": {
			seedExisting: true,
			wantErr:      true, wantExists: true,
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
				_, statErr := os.Stat(path)
				require.NoError(t, statErr, "the node should exist on disk")
				return
			}
			require.NoFileExists(t, path, "a failed creation must leave no node behind")
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &Custodian{basePath: tc.basePath}

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

// TestOpenRefusesAnUnstampableRoot pins that a root the custodian cannot stamp is refused
// rather than served. Everything the sub-tree holds is a credential the agent manages, so
// serving it unstamped would publish those to every unprivileged process in every instance.
// The statuses a volume without attribute support actually answers are measured: creating
// a node gives STATUS_EAS_NOT_SUPPORTED, adopting one already there gives
// STATUS_INVALID_DEVICE_REQUEST, and both must refuse.
func TestOpenRefusesAnUnstampableRoot(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// failStatus makes the root creation fail with this NT status.
		failStatus windows.NTStatus
		// preCreate leaves the directory on disk, so it is adopted rather than created.
		// The old fallback found it in place and succeeded, which is what hid the refusal.
		preCreate bool
	}{
		"a filesystem without attribute support":                {failStatus: windows.STATUS_EAS_NOT_SUPPORTED},
		"a filesystem that cannot store the attribute":          {failStatus: windows.STATUS_INVALID_DEVICE_REQUEST},
		"an unsupported operation":                              {failStatus: windows.STATUS_NOT_SUPPORTED},
		"a refused creation":                                    {failStatus: windows.STATUS_ACCESS_DENIED},
		"a rejected buffer":                                     {failStatus: windows.STATUS_INVALID_PARAMETER},
		"a filesystem without attribute support, adopted":       {failStatus: windows.STATUS_EAS_NOT_SUPPORTED, preCreate: true},
		"a filesystem that cannot store the attribute, adopted": {failStatus: windows.STATUS_INVALID_DEVICE_REQUEST, preCreate: true},
		"a refused creation over an existing directory":         {failStatus: windows.STATUS_ACCESS_DENIED, preCreate: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := filepath.Join(t.TempDir(), ".ubuntupro")
			if tc.preCreate {
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")
			}

			cust, err := OpenRefusingCreation(rootDir, uint32(tc.failStatus))
			require.Error(t, err, "a root that cannot be stamped must not be served")
			require.Nil(t, cust, "no custodian may be handed out for a refused root")
		})
	}
}

// TestStampingAnExistingNodeIsRefused pins the two places a node already on disk must be
// stamped in place rather than created stamped: the root an earlier run left behind, and a
// first-level sub-tree root the custodian adopts (ADR 2.01). Both open successfully and
// fail only at the stamp, which is the path NtCreateFile's EA buffer cannot reach, so
// neither is covered by refusing creation. A node adopted unstamped is a node no instance
// sees as root-owned, so the adoption has to fail rather than hand back a custodian.
func TestStampingAnExistingNodeIsRefused(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// failStatus is the status NtSetEaFile answers with.
		failStatus windows.NTStatus
		// subdir adopts a first-level sub-tree root instead of the custodian root.
		subdir bool
	}{
		"a root on a filesystem without attribute support":     {failStatus: windows.STATUS_EAS_NOT_SUPPORTED},
		"a root on a filesystem that cannot store it":          {failStatus: windows.STATUS_INVALID_DEVICE_REQUEST},
		"a root whose stamp is refused":                        {failStatus: windows.STATUS_ACCESS_DENIED},
		"a sub-tree on a filesystem without attribute support": {failStatus: windows.STATUS_EAS_NOT_SUPPORTED, subdir: true},
		"a sub-tree on a filesystem that cannot store it":      {failStatus: windows.STATUS_INVALID_DEVICE_REQUEST, subdir: true},
		"a sub-tree whose stamp is refused":                    {failStatus: windows.STATUS_ACCESS_DENIED, subdir: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			rootDir := filepath.Join(base, ".ubuntupro")

			if !tc.subdir {
				// The root has to be there already: a root being created carries the stamp
				// in the creation itself and never reaches NtSetEaFile.
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")

				cust, err := OpenRefusingStamp(rootDir, uint32(tc.failStatus))
				require.Error(t, err, "a root that cannot be stamped must not be served")
				require.Nil(t, cust, "no custodian may be handed out for a refused root")
				return
			}

			cust, err := Open(rootDir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			// Created while healthy, so the sub-tree root exists and the next Subdir
			// adopts it, which is the path that stamps in place.
			sub, err := cust.Subdir("sub")
			require.NoError(t, err, "Setup: could not create the sub-tree")
			require.NoError(t, sub.Close(), "Setup: could not close the sub-tree")

			cust.FailStamping(uint32(tc.failStatus))

			sub, err = cust.Subdir("sub")
			require.Error(t, err, "a sub-tree that cannot be stamped must not be served")
			require.Nil(t, sub, "no custodian may be handed out for a refused sub-tree")
		})
	}
}

// TestEnsureRootRefusesARedirectedRoot pins that a directory link standing where the root
// should be is refused rather than followed, with a real reparse point on disk rather than
// a seam. OBJ_DONT_REPARSE fails the creation on purpose, and the refusal must reach the
// caller as ErrPathEscapes: anything that adopted the link instead would root the custodian
// outside basePath, recording no identity and leaving setRoot nothing to compare against.
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
