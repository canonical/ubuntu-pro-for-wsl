//go:build linux

package securefiles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// platformSys guards its fields for the same reason the Windows one does: a custodian is
// shared between goroutines, and degraded in particular is read to decide whether a node
// can be verified at all.
type platformSys struct {
	mu       sync.Mutex
	xattr    xattrCalls
	root     *os.Root
	degraded bool
	// degradedCause records what first prevented stamping, for CheckProjection to relay.
	degradedCause string
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
	if err := os.MkdirAll(basePath, DirMode); err != nil {
		return nil, err
	}

	f, err := os.Open(basePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return &platformSys{xattr: xattr, degraded: !xattrsSupported(xattr, int(f.Fd()))}, nil //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
}

// newSubPlatformSys returns a platformSys for a sub-directory the parent custodian has
// already created. It does no path walk of its own: the node is reached through the
// parent's root, and re-resolving its absolute path here would step outside the
// containment the parent established. Degradation is inherited because it describes the
// filesystem, not the node.
func newSubPlatformSys(degraded bool) *platformSys {
	return &platformSys{degraded: degraded, xattr: realXattrCalls()}
}

// stampSubdir is a no-op: the Linux watermark is only ever applied to regular files, so
// there is nothing to stamp in place when a sub-directory is adopted. It exists to keep
// Subdir platform-agnostic, mirroring the Windows directory stamp.
func (s *platformSys) stampSubdir(string) error {
	return nil
}

// setRoot anchors the platform operations on the custodian's root: every node
// operation goes through it, so containment is enforced per syscall.
func (s *platformSys) setRoot(root *os.Root) error {
	s.root = root
	return nil
}

func (s *platformSys) Close() error {
	return nil
}

func (s *platformSys) createNode(rel string, isDir bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if isDir {
		return s.root.Mkdir(rel, DirMode)
	}

	f, err := s.root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
	if err != nil {
		return err
	}

	if !s.degraded {
		if err := stampNode(s.xattr, f); err != nil {
			closeErr := f.Close()
			if isXattrUnsupported(err) {
				// Filesystem does not support xattrs: mirror Windows degraded mode by
				// failing open rather than refusing to operate.
				s.degrade(fmt.Sprintf("could not stamp %s: %v", rel, err))
				return closeErr
			}
			return errors.Join(err, closeErr, s.root.Remove(rel))
		}
	}

	return f.Close()
}

func (s *platformSys) renameNode(oldRel, newRel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.degraded {
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
	}
	return s.root.Rename(oldRel, newRel)
}

func (s *platformSys) isDegraded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.degraded
}

// cause returns what first prevented stamping, or "" if nothing recorded it.
func (s *platformSys) cause() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.degradedCause
}

// isOwned reports whether the node carries the custodian's watermark and still
// has the same owner, group, and mode recorded at creation time. It never
// answers on behalf of a filesystem that cannot carry xattrs: there the query
// fails and ownership is unknowable rather than true, mirroring the Windows
// predicate. Callers must consult isDegraded first and decide what an
// unverifiable sub-tree means for them.
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

// xattrsSupported reports whether the filesystem behind fd can carry the watermark at
// all. Probing once, while the root is being established, is what lets every later read
// be a pure query: a predicate that discovered the answer mid-scan would have to mutate
// shared state from a read path, and would report a node as unverifiable for a reason
// that has nothing to do with that node. Listing is used rather than a write so the probe
// leaves no trace of its own.
func xattrsSupported(xattr xattrCalls, fd int) bool {
	_, err := xattr.list(fd, nil)
	return !isXattrUnsupported(err)
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
			// Whether the filesystem carries xattrs was settled when the root was
			// established, so this is not the place to decide it: report that the node
			// cannot be verified and leave the custodian's state alone.
			return false, fmt.Errorf("filesystem cannot carry the ownership watermark: %v", err)
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
