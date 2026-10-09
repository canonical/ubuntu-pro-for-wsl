//go:build linux

package securefiles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// platformSys guards its fields for the same reason the Windows one does: a custodian is
// shared between goroutines.
type platformSys struct {
	mu    sync.Mutex
	xattr xattrCalls
	root  *os.Root
	// rootDev and rootIno record the identity of the tree root as established at
	// construction. setRoot compares them against the root open() re-resolved, so a
	// link planted in the window between the two resolutions refuses the custodian
	// instead of redirecting it, the same tie Windows keeps with its recorded identity.
	// A construction that could not record one concedes once, at the first resolution.
	rootDev     uint64
	rootIno     uint64
	rootIDKnown bool
}

// watermarkXattr is the user namespace extended attribute used to stamp files
// created by the custodian. It acts as a platform mock for the NTFS extended
// attributes used on Windows. User xattrs require no privileges for files the
// process owns, which matches the test environment where files are created in
// test-owned temporary directories.
const watermarkXattr = "user.io.canonical.up4w.custodian.watermark"

// watermarkLen is the exact size of the watermark: owner, group and mode, each a
// big-endian uint32. A value of any other length did not come from stampNode, so it
// describes nothing and is treated as no watermark at all.
const watermarkLen = 12

// xattrCalls is the xattr surface the watermark path uses. It is a field of platformSys
// rather than a set of package-level variables so that no process-wide switch exists that
// could turn watermarking off for every custodian at once: it is set once at construction
// and never reassigned. Production wires realXattrCalls; the tests in this package wire a
// fake to stand in for a filesystem that does not carry user extended attributes.
type xattrCalls struct {
	set  func(fd int, attr string, data []byte, flags int) error
	get  func(fd int, attr string, dest []byte) (int, error)
	list func(fd int, dest []byte) (int, error)
}

// realXattrCalls returns the production implementation: the xattr syscalls themselves.
func realXattrCalls() xattrCalls {
	return xattrCalls{set: unix.Fsetxattr, get: unix.Fgetxattr, list: unix.Flistxattr}
}

func newPlatformSys(basePath string) (*platformSys, error) {
	return newPlatformSysWith(basePath, realXattrCalls())
}

// newPlatformSysWith is newPlatformSys with the xattr surface supplied rather than assumed.
func newPlatformSysWith(basePath string, xattr xattrCalls) (*platformSys, error) {
	basePath = filepath.Clean(basePath)
	// The parents are the agent's own directories and are established the ordinary
	// way, exactly as the Windows ensureRoot does with its parent handle. The tree
	// root itself is another matter: established relative to its parent without
	// following the leaf, because a link standing where the root should be redirects
	// the whole tree, and Open runs before there is a stamped tree to vouch for the
	// neighborhood. Windows refuses such a leaf by attribute (OBJ_DONT_REPARSE
	// answers ErrPathEscapes); the no-follow open here is its Linux counterpart.
	if err := os.MkdirAll(filepath.Dir(basePath), DirMode); err != nil {
		return nil, err
	}

	parent, err := os.OpenRoot(filepath.Dir(basePath))
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()

	leaf := filepath.Base(basePath)
	if err := parent.Mkdir(leaf, DirMode); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	// A pre-existing node is adopted only when it is a real directory: this open
	// refuses a link standing at the leaf instead of resolving to its target, and
	// that refusal is the escape verdict.
	f, err := parent.Open(leaf)
	if err != nil {
		return nil, refusedRootError(basePath, err)
	}
	defer func() { _ = f.Close() }()

	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return nil, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, fmt.Errorf("the tree root %s is not a directory", basePath)
	}

	// Refused here rather than carried: a sub-tree no instance sees as root-owned would
	// publish the agent's credentials to every unprivileged process in every instance.
	// The probe is weaker than the operation it predicts — it lists attributes on the
	// directory, while stamping sets one on a file — so createNode refuses on its own
	// evidence too.
	if err := probeXattrs(xattr, int(f.Fd())); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return nil, err
	}

	return &platformSys{
		xattr:       xattr,
		rootDev:     st.Dev,
		rootIno:     st.Ino,
		rootIDKnown: true,
	}, nil
}

// refusedRootError classifies the refusal of the tree root's leaf: a link standing
// there is the escape verdict, the same one Windows answers by attribute. Anything
// else is reported as the filesystem said it.
func refusedRootError(basePath string, err error) error {
	if isEscapeError(err) || errors.Is(err, unix.ELOOP) {
		return fmt.Errorf("the tree root %s is a link: %w", basePath, ErrPathEscapes)
	}
	return fmt.Errorf("could not establish the tree root %s: %w", basePath, err)
}

// newSubPlatformSys returns a platformSys for a sub-directory the parent custodian has
// already created. It does no path walk of its own: the node is reached through the
// parent's root, and re-resolving its absolute path here would step outside the
// containment the parent established.
func newSubPlatformSys(parent *platformSys) *platformSys {
	return &platformSys{xattr: parent.xattr}
}

// stampSubdir is a no-op: the Linux watermark is only ever applied to regular files, so
// there is nothing to stamp in place when a sub-directory is adopted. It exists to keep
// Subdir platform-agnostic, mirroring the Windows directory stamp.
func (s *platformSys) stampSubdir(string) error {
	return nil
}

// isReparsePoint reports whether the node standing at rel is a symbolic link, the only
// reparse kind a Linux filesystem carries. The no-follow open refuses links regardless
// of where they point, answering ELOOP, while a link escaping the root answers the
// os.Root path-escaping error before the no-follow flag is consulted; both mean a link.
// Any real node answers for itself.
func (s *platformSys) isReparsePoint(rel string) (bool, error) {
	f, err := s.root.OpenFile(rel, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err == nil {
		return false, f.Close()
	}
	if errors.Is(err, unix.ELOOP) || isEscapeError(err) {
		return true, nil
	}
	return false, err
}

// setRoot anchors the platform operations on the custodian's root: every node
// operation goes through it, so containment is enforced per syscall.
func (s *platformSys) setRoot(root *os.Root) error {
	// A second, independent resolution of the path the constructor already
	// established and held. The parent directory stays writable from inside an
	// instance (ADR 2.01), so between the two resolutions it can be made to point
	// elsewhere: the recorded identity ties them to one node, and a mismatch is a
	// failed verification rather than a redirect to serve. A construction that
	// could not record an identity (a sub-tree derived from a parent, or a test
	// build) is conceded once: nothing was recorded, so there is nothing to compare.
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return err
	}
	if s.rootIDKnown && (st.Dev != s.rootDev || st.Ino != s.rootIno) {
		return ErrRootReplaced
	}

	s.root = root
	return nil
}

func (s *platformSys) Close() error {
	return nil
}

// createNode creates and stamps a node relative to the custodian's root and returns it
// open, mirroring the Windows contract: the stamp rides on creation, a directory has no
// descriptor to hand out, and the caller owns the returned file.
func (s *platformSys) createNode(rel string, isDir bool) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if isDir {
		err := s.root.Mkdir(rel, DirMode)
		if errors.Is(err, os.ErrExist) {
			// An existing node is adopted only when it is a real directory: a link
			// standing at the name is reported instead of resolved, and deciding what
			// to do about a link is the caller's policy. The node is answered as
			// ErrExist and the caller adopts by handoff, mirroring Windows.
			d, oerr := s.root.OpenFile(rel, os.O_RDONLY|unix.O_NOFOLLOW, 0)
			if oerr != nil {
				return nil, oerr
			}
			return nil, errors.Join(d.Close(), os.ErrExist)
		}
		if err != nil {
			return nil, err
		}
		// A fresh Mkdir is trusted after this: os.Root resolves no leaf symlink here
		// (measured: Mkdir over one answers ErrExist without creating the target), and
		// TestSubdirReplacesAPlantedReparsePoint pins the promise on every platform.
		return nil, nil
	}

	f, err := s.root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
	if err != nil {
		return nil, err
	}

	if err := stampNode(s.xattr, f); err != nil {
		closeErr := f.Close()
		if isXattrUnsupported(err) {
			return nil, errors.Join(fmt.Errorf("could not stamp %s: %w", rel, ErrNoWatermarkSupport), closeErr, s.root.Remove(rel))
		}
		return nil, errors.Join(err, closeErr, s.root.Remove(rel))
	}

	return f, nil
}

func (s *platformSys) renameNode(oldRel, newRel string, expect *os.File) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := s.root.Open(oldRel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	// On Linux directories are not stamped; only regular files carry the watermark.
	if !st.IsDir() {
		owned, err := ownedByWatermark(s.xattr, int(f.Fd())) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		if err != nil {
			return err
		}
		if !owned {
			return ErrNotOwned
		}
	}

	// A caller that wrote through its own descriptor can name the node it expects to
	// publish. Ownership alone would let a name swapped in the write-publish window
	// move some other stamped node into the destination; the identity is what makes
	// the publish answer for exactly the bytes the caller wrote.
	if expect != nil && !sameNode(f, expect) {
		return fmt.Errorf("%s no longer holds the node the write went to: %w", oldRel, ErrNotOwned)
	}

	return s.root.Rename(oldRel, newRel)
}

// sameNode answers whether two open files are the same node: the same device and
// the same inode. An identity that cannot be read is not an identity, so a query
// failure answers no rather than guessing.
func sameNode(a, b *os.File) bool {
	var sa, sb unix.Stat_t
	if unix.Fstat(int(a.Fd()), &sa) != nil || unix.Fstat(int(b.Fd()), &sb) != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return false
	}
	return sa.Dev == sb.Dev && sa.Ino == sb.Ino
}

// isOwned reports whether the node carries the custodian's watermark and still
// has the same owner, group, and mode recorded at creation time. It never
// answers on behalf of a filesystem that cannot carry xattrs: Open refuses those, so
// a query failing here is an answer about this node.
func (s *platformSys) isOwned(rel string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := s.root.Open(rel)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	return ownedByWatermark(s.xattr, int(f.Fd())) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
}

// probeXattrs reports why the filesystem behind fd cannot carry the watermark, or nil
// when its support was established.
// Listing leaves no trace of its own, which is also its limit: it asks a directory whether
// attributes can be listed, while stamping asks a file to store one, so it is the first
// refusal rather than the only one.
func probeXattrs(xattr xattrCalls, fd int) error {
	_, err := xattr.list(fd, nil)
	if isXattrUnsupported(err) {
		return fmt.Errorf("%w: %w", ErrNoWatermarkSupport, err)
	}
	// Anything else is a refusal too: an unexplained probe failure is no evidence
	// that stamping works, and the constructor fails closed on it.
	if err != nil {
		return fmt.Errorf("could not establish watermark support: %w", err)
	}
	return nil
}

// remoteVolume always reports a local volume: the custodian's projection concerns are
// Windows-specific, and the Linux build exists to keep the cross-platform tests honest.
func remoteVolume(string) (remote bool, kind string) {
	return false, ""
}

// resolvedBasePath mirrors the Windows helper; there is nothing to resolve here.
func (s *platformSys) resolvedBasePath() string {
	return ""
}

// ownedByWatermark reports whether the open node still carries the watermark recorded
// for it at creation: the value must be present, the right size, and still describe the
// node's own owner, group and mode. Both the ownership predicate and the rename path go
// through here, because a rename that accepted a node the predicate would reject would
// let a tamperer who can write the attribute have the custodian publish their content.
// A missing watermark is not an error, only an answer; a filesystem that cannot carry
// one at all is, because then nothing about the node can be verified.
func ownedByWatermark(xattr xattrCalls, fd int) (bool, error) {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return false, err
	}

	buf := make([]byte, watermarkLen)
	n, err := xattr.get(fd, watermarkXattr, buf)
	if err != nil {
		if isXattrMissing(err) {
			return false, nil
		}
		if isXattrUnsupported(err) {
			// Settled when the root was established, so reaching this means the node
			// moved under us. Wrapped, not reworded, so a caller can recognise it.
			return false, fmt.Errorf("%w: %w", ErrNoWatermarkSupport, err)
		}
		return false, err
	}
	if n != watermarkLen {
		return false, nil
	}

	uid := binary.BigEndian.Uint32(buf[0:4])
	gid := binary.BigEndian.Uint32(buf[4:8])
	mode := binary.BigEndian.Uint32(buf[8:12])
	return uid == st.Uid && gid == st.Gid && mode == st.Mode, nil
}

// stampNode writes the custodian's watermark to the open file as a user
// namespace extended attribute. The value records the file's current owner,
// group, and mode so that later tampering with ownership or permissions
// invalidates it.
func stampNode(xattr xattrCalls, f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return err
	}

	b := make([]byte, watermarkLen)
	binary.BigEndian.PutUint32(b[0:4], st.Uid)
	binary.BigEndian.PutUint32(b[4:8], st.Gid)
	binary.BigEndian.PutUint32(b[8:12], st.Mode)
	return xattr.set(int(f.Fd()), watermarkXattr, b, 0) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
}

func isXattrMissing(err error) bool {
	return errors.Is(err, unix.ENODATA)
}

func isXattrUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
