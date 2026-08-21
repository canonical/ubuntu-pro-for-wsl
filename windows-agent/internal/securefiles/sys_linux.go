//go:build linux

package securefiles

import (
	"encoding/binary"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// watermarkXattr is the user namespace extended attribute used to stamp files
// created by the custodian. It acts as a platform mock for the NTFS extended
// attributes used on Windows. User xattrs require no privileges for files the
// process owns, which matches the test environment where files are created in
// test-owned temporary directories.
const watermarkXattr = "user.io.canonical.up4w.custodian.watermark"

// fsetxattr and fgetxattr alias the xattr syscalls so tests can swap them and
// simulate a filesystem without xattr support or a failing xattr call, the
// same role testNtSetEaFileResult plays in sys_windows.go.
var (
	fsetxattr = unix.Fsetxattr
	fgetxattr = unix.Fgetxattr
)

type platformSys struct {
	root      *os.Root
	degraded  bool
	mockOwned *bool
}

func newPlatformSys(basePath string) (*platformSys, error) {
	if err := os.MkdirAll(basePath, DirMode); err != nil {
		return nil, err
	}
	return &platformSys{}, nil
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
	if isDir {
		return s.root.Mkdir(rel, DirMode)
	}

	f, err := s.root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
	if err != nil {
		return err
	}

	if !s.degraded {
		if err := stampNode(f); err != nil {
			_ = f.Close()
			if isXattrUnsupported(err) {
				// Filesystem does not support xattrs: mirror Windows degraded mode by
				// failing open rather than refusing to operate.
				s.degraded = true
				return nil
			}
			return err
		}
	}

	return f.Close()
}

func (s *platformSys) renameNode(oldRel, newRel string) error {
	return s.root.Rename(oldRel, newRel)
}

func (s *platformSys) isDegraded() bool {
	return s.degraded
}

func (s *platformSys) setMockDegraded(degraded bool) {
	s.degraded = degraded
}

// isOwned reports whether the node carries the custodian's watermark and still
// has the same owner, group, and mode recorded at creation time.
func (s *platformSys) isOwned(rel string) (bool, error) {
	if s.mockOwned != nil {
		return *s.mockOwned, nil
	}
	if s.degraded {
		return true, nil
	}

	f, err := s.root.Open(rel)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return false, err
	}

	buf := make([]byte, 12)
	if _, err := fgetxattr(int(f.Fd()), watermarkXattr, buf); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		if isXattrMissing(err) {
			return false, nil
		}
		if isXattrUnsupported(err) {
			s.degraded = true
			return true, nil
		}
		return false, err
	}

	uid := binary.BigEndian.Uint32(buf[0:4])
	gid := binary.BigEndian.Uint32(buf[4:8])
	mode := binary.BigEndian.Uint32(buf[8:12])
	return uid == st.Uid && gid == st.Gid && mode == st.Mode, nil
}

func (s *platformSys) setMockOwned(owned *bool) {
	s.mockOwned = owned
}

// stampNode writes the custodian's watermark to the open file as a user
// namespace extended attribute. The value records the file's current owner,
// group, and mode so that later tampering with ownership or permissions
// invalidates it.
func stampNode(f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		return err
	}

	b := make([]byte, 12)
	binary.BigEndian.PutUint32(b[0:4], st.Uid)
	binary.BigEndian.PutUint32(b[4:8], st.Gid)
	binary.BigEndian.PutUint32(b[8:12], st.Mode)
	return fsetxattr(int(f.Fd()), watermarkXattr, b, 0) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
}

func isXattrMissing(err error) bool {
	return errors.Is(err, unix.ENODATA)
}

func isXattrUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
