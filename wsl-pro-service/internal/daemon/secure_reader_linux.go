package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// openat2Root implements rootFs using Linux openat2 with RESOLVE_NO_SYMLINKS,
// RESOLVE_BENEATH, and RESOLVE_NO_XDEV to atomically enforce confinement and
// reject symlinks at all levels.
type openat2Root struct {
	fd int
}

// productionFSNames lists the only filesystem types trusted for the root of the
// Public Directory: the WSL2 Windows drive projection, mounted as 9p (default)
// or virtiofs (opt-in via .wslconfig). Matching is exact: user-space FUSE mounts
// appear in mountinfo as "fuse", "fuseblk" or "fuse.<subtype>", and the subtype
// name is attacker-chosen, so anything but a whole-string match would be unsound.
var productionFSNames = []string{"9p", "virtiofs"}

// allowedFSNames is the effective allowlist: a copy of productionFSNames that
// test helpers extend via AllowFSNames so the real root-open path can run on
// the filesystems found on developer machines and CI runners.
var allowedFSNames = newFSNameSet(productionFSNames)

func newFSNameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// OpenHow policies for opening directory and file nodes securely under openat2.
// Both policies strictly forbid symlink traversal, path escapes, and mount-point crossing.
var (
	confinedDirOpenHow = unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH | unix.RESOLVE_NO_XDEV,
	}
	confinedFileOpenHow = unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_NONBLOCK | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH | unix.RESOLVE_NO_XDEV,
	}
)

// openRootOS is the production openRoot seam used by defaultSecureReader on Unix.
// It opens the named directory with O_NOFOLLOW and O_DIRECTORY, ensuring it is a real
// directory and not a symlink, and verifies that the filesystem it lives on is
// allowlisted.
//
// The filesystem type cannot come from statfs magic numbers: virtiofs is a FUSE
// mount and reports FUSE_SUPER_MAGIC just like any attacker-controlled userspace
// FUSE mount. Instead, the mount is identified through the kernel mount table:
// the descriptor pins the mount (immune to concurrent mount/unmount races), its
// mount ID is read from /proc/self/fdinfo, and the filesystem type is looked up
// in /proc/self/mountinfo. Any anomaly fails closed.
func openRootOS(path string) (rootFs, error) {
	clean := filepath.Clean(path)

	fd, err := unix.Open(clean, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}

	fstype, mountPoint, err := mountFSType(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("could not determine the filesystem of root %q: %v", clean, err)
	}

	if !allowedFSNames[fstype] {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("refusing untrusted filesystem %q at %q for root %q", fstype, mountPoint, clean)
	}

	return &openat2Root{fd: fd}, nil
}

// mountFSType reports the filesystem type and mount point of the mount containing fd.
func mountFSType(fd int) (fsType, mountPoint string, err error) {
	fdinfo, err := os.ReadFile("/proc/self/fdinfo/" + strconv.Itoa(fd))
	if err != nil {
		return "", "", err
	}

	mntID, err := parseMntID(string(fdinfo))
	if err != nil {
		return "", "", err
	}

	mountinfo, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", "", err
	}

	return fstypeForMountID(string(mountinfo), mntID)
}

// parseMntID extracts the mount ID a descriptor belongs to, as reported by
// /proc/self/fdinfo entries of the form "mnt_id:\t<id>". A missing or invalid
// entry is an error: callers must fail closed on it.
func parseMntID(fdinfo string) (int, error) {
	for line := range strings.SplitSeq(fdinfo, "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found || name != "mnt_id" {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, fmt.Errorf("invalid mnt_id %q in fdinfo: %v", value, err)
		}
		return id, nil
	}
	return 0, errors.New("no mnt_id in fdinfo")
}

// fstypeForMountID scans mountinfo contents for the entry with the given mount ID
// and returns its filesystem type and mount point. mountinfo lines have the form:
//
//	mntID parentID major:minor root mountpoint [options...] - fstype source [superopts...]
//
// Only whole fields are inspected, never paths, so the octal escapes mountinfo
// uses for special characters in paths never apply.
func fstypeForMountID(mountinfo string, mntID int) (fsType, mountPoint string, err error) {
	want := strconv.Itoa(mntID)
	for line := range strings.SplitSeq(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != want {
			continue
		}
		if len(fields) < 5 {
			return "", "", fmt.Errorf("malformed mountinfo entry for mount %d", mntID)
		}
		for i, field := range fields {
			if field != "-" {
				continue
			}
			if i+1 >= len(fields) {
				return "", "", fmt.Errorf("malformed mountinfo entry for mount %d", mntID)
			}
			return fields[i+1], fields[4], nil
		}
		return "", "", fmt.Errorf("malformed mountinfo entry for mount %d", mntID)
	}
	return "", "", fmt.Errorf("mount %d not found in mountinfo", mntID)
}

func (r *openat2Root) Close() error {
	if r.fd >= 0 {
		err := unix.Close(r.fd)
		r.fd = -1
		return err
	}
	return nil
}

func (r *openat2Root) Lstat(name string) (fileStat, error) {
	if r.fd < 0 {
		return fileStat{}, errors.New("root is closed")
	}

	clean := filepath.Clean(name)
	if clean == "." {
		var stat unix.Stat_t
		if err := unix.Fstat(r.fd, &stat); err != nil {
			return fileStat{}, err
		}
		return fileStat{Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
	}

	if !filepath.IsLocal(name) {
		return fileStat{}, fmt.Errorf("path %q is not local to root", name)
	}

	dir := filepath.Dir(clean)
	base := filepath.Base(clean)

	parentFd := r.fd
	if dir != "." {
		fd, err := unix.Openat2(r.fd, dir, &confinedDirOpenHow)
		if err != nil {
			return fileStat{}, err
		}
		defer unix.Close(fd)
		parentFd = fd
	}

	var stat unix.Stat_t
	if err := unix.Fstatat(parentFd, base, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fileStat{}, err
	}
	return fileStat{Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
}

type openat2File struct {
	*os.File
	fd int
}

func (f *openat2File) Stat() (fileStat, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(f.fd, &stat); err != nil {
		return fileStat{}, err
	}
	return fileStat{Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
}

func (r *openat2Root) Open(name string) (confinedFile, error) {
	if r.fd < 0 {
		return nil, errors.New("root is closed")
	}
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("path %q is not local to root", name)
	}

	clean := filepath.Clean(name)
	fd, err := unix.Openat2(r.fd, clean, &confinedFileOpenHow)
	if err != nil {
		return nil, err
	}
	// #nosec G115 // If err is nil, openat2 returned a positive descriptor, no risk of overflows.
	return &openat2File{File: os.NewFile(uintptr(fd), clean), fd: fd}, nil
}
