//go:build windows

// Mechanism tests over the Windows platform layer: the NT syscall seams
// reproduce filesystems this machine does not have, and pin ADR 2.02
// throughout — a sub-tree that cannot be stamped is refused rather than
// served unstamped. The decisions that do not need the mechanism are tested
// cross-platform, over the fake platformSys in sys_fake_internal_test.go.

package securefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestCreateStamped covers the one decision the node-creating call makes on
// its own: a node already standing at the name is refused rather than
// silently adopted, whatever the filesystem answered, and a fresh name is
// created stamped. The statuses a creation cannot stamp are pinned where the
// same seam and translation run against the root, in
// TestEstablishRootRefusesWhatCreationCannotStamp.
func TestCreateStamped(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// seedExisting puts a node at the path before the call.
		seedExisting bool

		wantErr    bool
		wantExists bool
	}{
		"a file is created and stamped": {wantExists: true},
		"an existing node is refused, not adopted": {
			seedExisting: true,
			wantErr:      true, wantExists: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			const node = "node"
			if tc.seedExisting {
				seed, err := cust.sys.createStamped(node, false)
				require.NoError(t, err, "Setup: could not seed the existing node")
				if seed != nil {
					require.NoError(t, seed.Close(), "Setup: could not close the seeded node")
				}
			}

			created, err := cust.sys.createStamped(node, false)
			if created != nil {
				defer func() { _ = created.Close() }()
			}
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

// TestRemoteVolumeClassification pins how the public directory's volume is
// classified. The distinction matters because a remote volume is the one case
// where the custodian reports healthy while the guarantee may not hold:
// stamping succeeds on the server, but what instances see is a projection of
// a share. UNC paths must be recognised without calling GetDriveType, which
// blocks on name resolution for an unreachable server and then reports
// DRIVE_NO_ROOT_DIR regardless.
func TestRemoteVolumeClassification(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path string
		// driveType is what the seam answers for the volume root. Zero, the
		// default, wires the real query: only a machine with a mapped drive
		// can answer DRIVE_REMOTE, so the mapped verdict is pinned by the fake.
		driveType  uint32
		wantRemote bool
		wantKind   volumeKind
	}{
		"a local drive":                 {path: `C:\Users\someone\.ubuntupro`, driveType: windows.DRIVE_FIXED},
		"a relative path has no volume": {path: `.ubuntupro`},
		"a UNC path":                    {path: `\\server\share\Users\someone\.ubuntupro`, wantRemote: true, wantKind: volumeUNC},
		"an unreachable UNC path":       {path: `\\no-such-host-xyz\share\.ubuntupro`, wantRemote: true, wantKind: volumeUNC},
		"a mapped network drive":        {path: `C:\Users\someone\.ubuntupro`, driveType: windows.DRIVE_REMOTE, wantRemote: true, wantKind: volumeMappedDrive},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nt := realNtCalls()
			if tc.driveType != 0 {
				nt.getDriveType = func(*uint16) uint32 { return tc.driveType }
			}
			sys := newWindowsSysWith(t.TempDir(), nt)
			t.Cleanup(func() { _ = sys.release() })

			start := time.Now()
			remote, kind := sys.remoteVolume(tc.path)
			elapsed := time.Since(start)

			require.Equal(t, tc.wantRemote, remote, "volume should be classified as remote=%v", tc.wantRemote)
			require.Equal(t, tc.wantKind, kind)

			// Classification runs on the startup path, so it must never reach out to
			// the network: an unreachable server would otherwise stall the agent.
			require.Less(t, elapsed, 250*time.Millisecond, "classification must not perform a network lookup")
		})
	}
}

// TestNTPathEncodingIsRefused pins that a relative path the NT layer cannot
// encode is refused rather than silently truncated at the NUL. NT counted
// strings carry an explicit length, so a truncating implementation would let a
// caller name "good\x00evil" and have the kernel act on "good" instead. Every
// case therefore seeds a real node called "good": if the NUL were dropped, the
// call would succeed against that node instead of failing. Every NT open goes
// through relativeAttributes, so the cases are the entry points that name a
// node.
func TestNTPathEncodingIsRefused(t *testing.T) {
	t.Parallel()

	const bad = "good\x00evil"

	testCases := map[string]struct {
		seedFile bool
		seedDir  bool

		call  func(c *Custodian) error
		after func(t *testing.T, dir string)
	}{
		"ownership check": {
			seedFile: true,
			call: func(c *Custodian) error {
				owned, err := c.IsOwned(bad)
				if err == nil && owned {
					return nil // truncated onto the seeded node
				}
				return err
			},
		},
		"node creation": {
			call: func(c *Custodian) error {
				f, err := c.sys.createStamped(bad, false)
				if f != nil {
					_ = f.Close()
				}
				return err
			},
			after: func(t *testing.T, dir string) {
				t.Helper()
				require.NoFileExists(t, filepath.Join(dir, "good"), "a truncated path must not create the shorter node")
			},
		},
		"rename source": {
			seedFile: true,
			call:     func(c *Custodian) error { return c.Rename(bad, "moved.txt") },
			after: func(t *testing.T, dir string) {
				t.Helper()
				require.FileExists(t, filepath.Join(dir, "good"), "a truncated path must not move the shorter node")
			},
		},
		"sub-directory stamp": {
			seedDir: true,
			call:    func(c *Custodian) error { return c.sys.stampDir(bad) },
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

			require.Error(t, tc.call(cust), "an unencodable path must be refused, never truncated")

			if tc.after != nil {
				tc.after(t, dir)
			}
		})
	}
}

// TestRenameReplace covers the two conditions the Windows rename has to
// survive on its own: a destination leaf it cannot encode, which is checked
// only after the source is open, and a filesystem with no
// FILE_RENAME_INFORMATION_EX at all, where the rename must still happen
// through the original information class rather than be reported as a
// failure. The fallback decision itself is tested cross-platform, over the
// fake platformSys; this is the real classes against the real kernel.
func TestRenameReplace(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			dir := t.TempDir()
			cust, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer cust.Close()

			require.NoError(t, cust.WriteFile("src.txt", []byte("payload")), "Setup: could not seed the source")
			if tc.withoutRenameInfoEx {
				cust.withoutRenameInfoEx()
			}

			err = cust.Rename("src.txt", tc.to)
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

// TestIdentityOf pins the best-effort contract of the identity check: a handle
// it cannot interrogate yields "unknown" rather than a zero identity, which
// would compare equal to another unknown one and wrongly certify a swapped
// root.
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

			// establishRoot closes its handle, so a live one has to come from elsewhere.
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

// TestEnsureRootRejectsUnusableBasePath pins that a base path the platform
// cannot establish is reported at construction, where it can still be acted
// on.
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

			_, err := Open(tc.basePath)
			require.Error(t, err, "an unusable base path must be refused")
		})
	}
}

// TestEstablishRootRefusesWhatCreationCannotStamp pins that a root whose
// creation the volume refuses — for the measured statuses a volume without
// attribute support answers with — is refused rather than served. Everything
// the sub-tree holds is a credential the agent manages, so serving it
// unstamped would publish those to every unprivileged process in every
// instance. The refusal policy itself is tested cross-platform, over the fake
// platformSys; this is the statuses through the real create call.
func TestEstablishRootRefusesWhatCreationCannotStamp(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// failStatus makes the root creation fail with this NT status.
		failStatus windows.NTStatus
		// preCreate leaves the directory on disk, so it is adopted rather than created.
		// The old fallback found it in place and succeeded, which is what hid the refusal.
		preCreate bool
	}{
		"a filesystem without attribute support":          {failStatus: windows.STATUS_EAS_NOT_SUPPORTED},
		"a filesystem that cannot store the attribute":    {failStatus: windows.STATUS_INVALID_DEVICE_REQUEST},
		"an unsupported operation":                        {failStatus: windows.STATUS_NOT_SUPPORTED},
		"a refused creation":                              {failStatus: windows.STATUS_ACCESS_DENIED},
		"a rejected buffer":                               {failStatus: windows.STATUS_INVALID_PARAMETER},
		"a filesystem without attribute support, adopted": {failStatus: windows.STATUS_EAS_NOT_SUPPORTED, preCreate: true},
		"a refused creation over an existing directory":   {failStatus: windows.STATUS_ACCESS_DENIED, preCreate: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := filepath.Join(t.TempDir(), ".ubuntupro")
			if tc.preCreate {
				require.NoError(t, os.MkdirAll(rootDir, DirMode), "Setup: could not pre-create the root")
			}

			cust, err := openRefusingCreation(rootDir, uint32(tc.failStatus))
			require.Error(t, err, "a root that cannot be stamped must not be served")
			require.Nil(t, cust, "no custodian may be handed out for a refused root")
		})
	}
}

// TestStampingAnExistingNodeIsRefused pins the two places a node already on
// disk must be stamped in place rather than created stamped: the root an
// earlier run left behind, and a first-level sub-tree root the custodian
// adopts (ADR 2.01). Both open successfully and fail only at the stamp, which
// is the path NtCreateFile's EA buffer cannot reach, so neither is covered by
// refusing creation. A node adopted unstamped is a node no instance sees as
// root-owned, so the adoption has to fail rather than hand back a custodian.
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

				cust, err := openRefusingStamp(rootDir, uint32(tc.failStatus))
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

			cust.failStamping(uint32(tc.failStatus))

			sub, err = cust.Subdir("sub")
			require.Error(t, err, "a sub-tree that cannot be stamped must not be served")
			require.Nil(t, sub, "no custodian may be handed out for a refused sub-tree")
		})
	}
}

// TestEnsureRootRefusesARedirectedRoot pins that a directory link standing
// where the root should be is refused rather than followed, with a real
// reparse point on disk rather than a seam. OBJ_DONT_REPARSE fails the
// creation on purpose, and the refusal must reach the caller as
// ErrPathEscapes: anything that adopted the link instead would root the
// custodian outside basePath.
func TestEnsureRootRefusesARedirectedRoot(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// redirect puts a directory symlink where the root would be created, standing in
		// for any reparse point: a junction planted from inside an instance behaves alike.
		redirect bool
	}{
		"a root the custodian creates itself":       {},
		"a directory link standing in for the root": {redirect: true},
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

			sys := newPlatformSys(rootPath)
			_, _, err := sys.establishRoot()
			if tc.redirect {
				require.ErrorIs(t, err, ErrPathEscapes, "a redirected root must be refused")

				entries, err := os.ReadDir(outside)
				require.NoError(t, err, "Setup: could not list the link target")
				require.Empty(t, entries, "nothing may be created through the link")
				return
			}
			require.NoError(t, err, "the custodian should have established its root")
			require.NoError(t, sys.release())
		})
	}
}

// TestCheckProjectionReportsARedirectedRoot pins the report a root the kernel
// resolves into a share receives, and the shapes of the final-path query that
// classify it. A share is a condition no CI machine can produce, so the
// ntCalls seam reproduces the answers GetFinalPathNameByHandle can give: the
// redirected path itself, one that over-answers the first buffer, and a
// refusal.
func TestCheckProjectionReportsARedirectedRoot(t *testing.T) {
	t.Parallel()

	const redirected = `\\?\UNC\server\share\publicdir`

	testCases := map[string]struct {
		// finalPath is what the seam answers for the root handle.
		finalPath func(buf *uint16, maxLen uint32) (uint32, error)

		// wantErr is empty for a root judged healthy: with the query refusing, the
		// classification falls back to the name the custodian was handed.
		wantErr      error
		wantContains []string
	}{
		"a root the kernel resolves into a share": {
			finalPath:    answerRedirected(redirected),
			wantErr:      ErrRemoteVolume,
			wantContains: []string{"resolving to", redirected},
		},
		"a final path that over-answers the first buffer": {
			finalPath:    answerAfterRetry(redirected),
			wantErr:      ErrRemoteVolume,
			wantContains: []string{"resolving to", redirected},
		},
		"a final path the kernel refuses to answer": {
			finalPath: func(_ *uint16, _ uint32) (uint32, error) {
				return 0, windows.ERROR_ACCESS_DENIED
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			nt := realNtCalls()
			nt.finalPath = func(_ windows.Handle, buf *uint16, maxLen uint32) (uint32, error) {
				return tc.finalPath(buf, maxLen)
			}
			sys := newWindowsSysWith(tmp, nt)
			_, _, err := sys.establishRoot()
			require.NoError(t, err, "Setup: could not establish the root")
			t.Cleanup(func() { _ = sys.release() })

			c := &Custodian{basePath: tmp, sys: sys}

			gaps := c.CheckProjection()
			if tc.wantErr == nil {
				require.NoError(t, gaps, "a root the query cannot resolve must be judged by the name it was handed")
				return
			}
			require.ErrorIs(t, gaps, tc.wantErr, "CheckProjection must report the redirected root")
			for _, want := range tc.wantContains {
				require.ErrorContains(t, gaps, want, "CheckProjection must carry what was found")
			}
		})
	}
}

// TestSubdirFailsWhenTheHoldIsRefused pins the sub-custodian's hold: the
// hold a sub-tree root needs is taken beside the node the verification just
// answered for, and a volume that refuses that open refuses the sub-tree
// rather than serving it unheld — a sub-tree root that can be renamed or
// removed while it serves is the runtime root-swap the veto exists to close
// (ADR 2.01).
func TestSubdirFailsWhenTheHoldIsRefused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	realNt := realNtCalls()
	nt := realNtCalls()
	nt.createFile = func(handle *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, attrs, share, disp, opts uint32, ea []byte) error {
		// The hold is the layer's only create with no stamp buffer and a
		// plain FILE_OPEN disposition; establishment answers FILE_OPEN_IF and
		// node creation answers FILE_CREATE, so both go through untouched.
		if disp == windows.FILE_OPEN && len(ea) == 0 {
			return windows.STATUS_SHARING_VIOLATION
		}
		return realNt.createFile(handle, access, oa, iosb, attrs, share, disp, opts, ea)
	}

	c, err := open(dir, func(string) platformSys { return newWindowsSysWith(dir, nt) })
	require.NoError(t, err, "Setup: could not open the custodian")
	defer func() { _ = c.Close() }()

	_, err = c.Subdir("sub")
	require.Error(t, err, "a sub-tree whose hold is refused must not be handed out")
}

// TestReadStampSurfacesAnUndecodableStamp pins that a stamp the kernel hands
// back malformed — a name length beyond the buffer, as a corrupt or tampered
// EA list can read — is not a verdict about the node: the decode failure
// surfaces rather than being answered as not owned.
func TestReadStampSurfacesAnUndecodableStamp(t *testing.T) {
	t.Parallel()

	// A name length far beyond the buffer, in the fixed header's place.
	garbage := make([]byte, 8)
	garbage[5] = 200

	// The node is stamped by a healthy custodian first, because the write
	// itself verifies its own stamp: the garbage query would refuse it just
	// as rightly as the read below does. The tree survives the custodian —
	// the stamp is on disk — so the faked one adopts it.
	dir := t.TempDir()
	seed, err := Open(dir)
	require.NoError(t, err, "Setup: could not open the seeding custodian")
	require.NoError(t, seed.WriteFile("f", []byte("x")), "Setup: could not write the stamped node")
	require.NoError(t, seed.Close(), "Setup: could not close the seeding custodian")

	nt := realNtCalls()
	nt.queryEaFile = func(_ windows.Handle, buf []byte) (int, error) {
		return copy(buf, garbage), nil
	}
	cust, err := open(dir, func(p string) platformSys { return newWindowsSysWith(p, nt) })
	require.NoError(t, err, "Setup: could not open the custodian")
	defer cust.Close()

	_, err = cust.IsOwned("f")
	require.Error(t, err, "a stamp that cannot be decoded is not a verdict about the node")
	require.NotErrorIs(t, err, ErrNotOwned, "the refusal must not mistake a broken decode for a foreign node")
}

// TestMapNtStatus pins the whole translation table: the two conditions the
// custodian names for itself, the deliberate exclusion of
// STATUS_INVALID_PARAMETER, and the passthrough of everything else. The
// statuses for the watermark column are measured against a volume without
// attribute support, not assumed.
func TestMapNtStatus(t *testing.T) {
	t.Parallel()

	require.NoError(t, mapNtStatus(nil), "no error maps to no error")

	for _, st := range []windows.NTStatus{windows.STATUS_STOPPED_ON_SYMLINK, windows.STATUS_REPARSE_POINT_ENCOUNTERED} {
		require.ErrorIs(t, mapNtStatus(st), ErrPathEscapes,
			"a symlink crossing the custodian root answers the escape sentinel")
	}

	for _, st := range []windows.NTStatus{windows.STATUS_EAS_NOT_SUPPORTED, windows.STATUS_INVALID_DEVICE_REQUEST, windows.STATUS_NOT_SUPPORTED} {
		require.ErrorIs(t, mapNtStatus(st), ErrNoWatermarkSupport,
			"a volume that cannot carry the watermark answers the refusal sentinel")
	}

	// STATUS_INVALID_PARAMETER is excluded on purpose: no such volume returns
	// it, but our own wrong buffer or flags would, and reading that as a
	// property of the filesystem would blame the machine for our mistake.
	require.NotErrorIs(t, mapNtStatus(windows.STATUS_INVALID_PARAMETER), ErrNoWatermarkSupport,
		"a rejected buffer must stay our own failure, loud and unmapped")

	// Anything else is translated to its errno, not swallowed and not
	// reclassified.
	require.Error(t, mapNtStatus(windows.STATUS_ACCESS_DENIED), "a refused creation surfaces as an error")

	// A non-NT error passes through untouched, so the caller's identity and
	// message survive.
	plain := errors.New("plain failure")
	require.Same(t, plain, mapNtStatus(plain), "a non-NT error must pass through untouched")
}

// openRefusingStamp opens a custodian on a platform whose extended-attribute
// writes fail from the outset, standing in for a volume that refuses them.
func openRefusingStamp(basePath string, status uint32) (*Custodian, error) {
	return open(basePath, func(p string) platformSys {
		nt := realNtCalls()
		nt.setEaFile = func(windows.Handle, []byte) error { return windows.NTStatus(status) }
		return newWindowsSysWith(p, nt)
	})
}

// openRefusingCreation opens a custodian whose root creation fails with
// status, standing in for a volume or an ACL that refuses the node the stamp
// needs.
func openRefusingCreation(basePath string, status uint32) (*Custodian, error) {
	return open(basePath, func(p string) platformSys {
		nt := realNtCalls()
		nt.createFile = func(*windows.Handle, uint32, *windows.OBJECT_ATTRIBUTES, *windows.IO_STATUS_BLOCK, uint32, uint32, uint32, uint32, []byte) error {
			return windows.NTStatus(status)
		}
		return newWindowsSysWith(p, nt)
	})
}

// windowsLayer answers the custodian's OS layer as the Windows implementation,
// for the helpers that reach its seams. A custodian opened through open()
// with the production constructor always has one.
func windowsLayer(c *Custodian) *windowsSys {
	sys, ok := c.sys.(*windowsSys)
	if !ok {
		panic("the custodian's OS layer is not the Windows implementation")
	}
	return sys
}

// failStamping makes every later extended-attribute write on this custodian
// fail.
func (c *Custodian) failStamping(status uint32) {
	if c.sys == nil {
		return
	}
	s := windowsLayer(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nt.setEaFile = func(windows.Handle, []byte) error { return windows.NTStatus(status) }
}

// withoutRenameInfoEx makes the first rename attempt report the information
// class as unimplemented, as a volume predating
// FILE_RENAME_INFORMATION_EX does.
func (c *Custodian) withoutRenameInfoEx() {
	if c.sys == nil {
		return
	}
	s := windowsLayer(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	passthrough := s.nt.setInformationFile
	s.nt.setInformationFile = func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error {
		if class == fileRenameInformationEx {
			return windows.STATUS_NOT_IMPLEMENTED
		}
		return passthrough(h, iosb, buf, class)
	}
}

// answerRedirected answers the final-path query with the given path: the NT
// form GetFinalPathNameByHandle hands back for a directory reached through a
// link into a share.
func answerRedirected(path string) func(buf *uint16, maxLen uint32) (uint32, error) {
	utf, err := windows.UTF16FromString(path)
	if err != nil {
		panic(err) // a compile-time constant path; the conversion cannot fail for it
	}
	return func(buf *uint16, maxLen uint32) (uint32, error) {
		copy(unsafe.Slice(buf, maxLen), utf) //#nosec G103 // maxLen is the buffer's own length, so the overlay stays inside the caller's buffer.
		return uint32(len(utf)), nil         //#nosec G115 // length of a small path buffer; always fits in 32 bits.
	}
}

// answerAfterRetry answers the first query with the buffer-too-small
// refusal naming the size the caller must grow to, then answers the retry
// normally: the safety-net loop in finalPath.
func answerAfterRetry(path string) func(buf *uint16, maxLen uint32) (uint32, error) {
	inner := answerRedirected(path)
	calls := 0
	return func(buf *uint16, maxLen uint32) (uint32, error) {
		calls++
		if calls == 1 {
			return windows.MAX_LONG_PATH + 1, windows.ERROR_INSUFFICIENT_BUFFER
		}
		return inner(buf, maxLen)
	}
}
