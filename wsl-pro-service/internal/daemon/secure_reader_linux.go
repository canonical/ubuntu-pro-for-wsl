package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// openat2Root implements rootFs using Linux openat2 with RESOLVE_NO_SYMLINKS,
// RESOLVE_BENEATH, and RESOLVE_NO_XDEV to atomically enforce confinement and
// reject symlinks at all levels.
type openat2Root struct {
	fd   int
	path string
}

// Production allowed filesystem magic numbers for WSL Windows-mounts.
const (
	Magic9P       = unix.V9FS_MAGIC // 0x01021997
	MagicVirtioFS = 0x5a657366      // virtiofs
)

// allowedFSTypes contains the filesystem types permitted for root directories.
// In production, only 9p and virtiofs are allowed. Tests can extend this via
// AllowFSTypes during test package initialization.
var allowedFSTypes = map[int64]string{
	Magic9P:       "9p",
	MagicVirtioFS: "virtiofs",
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
// directory and not a symlink, and verifies that the filesystem type is allowlisted.
func openRootOS(path string) (rootFs, error) {
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	base := filepath.Base(clean)

	var (
		fd  int
		err error
	)

	// If it has no parent
	if parent == clean || clean == "/" {
		how := unix.OpenHow{
			Flags:   confinedDirOpenHow.Flags,
			Resolve: unix.RESOLVE_NO_SYMLINKS,
		}
		fd, err = unix.Openat2(unix.AT_FDCWD, clean, &how)
		if err != nil {
			return nil, err
		}
	} else {
		parentFd, pErr := unix.Open(parent, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if pErr != nil {
			return nil, pErr
		}
		defer unix.Close(parentFd)

		fd, err = unix.Openat2(parentFd, base, &confinedDirOpenHow)
		if err != nil {
			return nil, err
		}
	}

	var statfs unix.Statfs_t
	if err := unix.Fstatfs(fd, &statfs); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed to determine filesystem type: %v", err)
	}

	//nolint:unconvert // statfs.Type is int32 on 32-bit platforms, but int64 on 64-bit platforms.
	if _, ok := allowedFSTypes[int64(statfs.Type)]; !ok {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("refusing untrusted filesystem type 0x%x for root %q", statfs.Type, clean)
	}

	return &openat2Root{fd: fd, path: clean}, nil
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
		return fileStat{Name: ".", Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
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
	return fileStat{Name: base, Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
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
	return fileStat{Name: filepath.Base(f.Name()), Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid}, nil
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
