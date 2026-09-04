package daemon

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// ownershipOf extracts the UID and GID of the file described by info.
func ownershipOf(info fs.FileInfo) (uid, gid uint32, err error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected stat type %T", info.Sys())
	}

	return stat.Uid, stat.Gid, nil
}

// openat2Root implements rootFs using Linux openat2 with RESOLVE_NO_SYMLINKS and
// RESOLVE_BENEATH to atomically enforce confinement and reject symlinks at all levels.
type openat2Root struct {
	fd   int
	path string
}

// openRootOS is the production openRoot seam used by defaultSecureReader on Unix.
// It opens the named directory with O_NOFOLLOW and O_DIRECTORY, ensuring it is a real
// directory and not a symlink.
func openRootOS(path string) (rootFs, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return &openat2Root{fd: fd, path: path}, nil
}

func (r *openat2Root) Close() error {
	if r.fd >= 0 {
		err := unix.Close(r.fd)
		r.fd = -1
		return err
	}
	return nil
}

func (r *openat2Root) Lstat(name string) (fs.FileInfo, error) {
	if r.fd < 0 {
		return nil, errors.New("root is closed")
	}

	clean := filepath.Clean(name)
	if clean == "." {
		var stat unix.Stat_t
		if err := unix.Fstat(r.fd, &stat); err != nil {
			return nil, err
		}
		return fileInfoFromStat(".", &stat), nil
	}

	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("path %q is not local to root", name)
	}

	dir := filepath.Dir(clean)
	base := filepath.Base(clean)

	parentFd := r.fd
	if dir != "." {
		how := &unix.OpenHow{
			Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
			Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
		}
		fd, err := unix.Openat2(r.fd, dir, how)
		if err != nil {
			return nil, err
		}
		defer unix.Close(fd)
		parentFd = fd
	}

	var stat unix.Stat_t
	if err := unix.Fstatat(parentFd, base, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	return fileInfoFromStat(base, &stat), nil
}

func (r *openat2Root) Open(name string) (io.ReadCloser, error) {
	if r.fd < 0 {
		return nil, errors.New("root is closed")
	}
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("path %q is not local to root", name)
	}

	clean := filepath.Clean(name)
	how := &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
	}

	fd, err := unix.Openat2(r.fd, clean, how)
	if err != nil {
		return nil, err
	}
	if fd < 0 {
		return nil, errors.New("invalid file descriptor")
	}
	return os.NewFile(uintptr(fd), clean), nil
}

type statFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     syscall.Stat_t
}

func (s statFileInfo) Name() string       { return s.name }
func (s statFileInfo) Size() int64        { return s.size }
func (s statFileInfo) Mode() fs.FileMode  { return s.mode }
func (s statFileInfo) ModTime() time.Time { return s.modTime }
func (s statFileInfo) IsDir() bool        { return s.isDir }
func (s statFileInfo) Sys() any           { return &s.sys }

func fileInfoFromStat(name string, stat *unix.Stat_t) fs.FileInfo {
	sys := syscall.Stat_t{
		Dev:     stat.Dev,
		Ino:     stat.Ino,
		Nlink:   stat.Nlink,
		Mode:    stat.Mode,
		Uid:     stat.Uid,
		Gid:     stat.Gid,
		Rdev:    stat.Rdev,
		Size:    stat.Size,
		Blksize: stat.Blksize,
		Blocks:  stat.Blocks,
	}

	m := fs.FileMode(stat.Mode & 0777)
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		m |= fs.ModeDir
	case unix.S_IFLNK:
		m |= fs.ModeSymlink
	case unix.S_IFIFO:
		m |= fs.ModeNamedPipe
	case unix.S_IFSOCK:
		m |= fs.ModeSocket
	case unix.S_IFCHR:
		m |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFBLK:
		m |= fs.ModeDevice
	}

	return statFileInfo{
		name:    filepath.Base(name),
		size:    stat.Size,
		mode:    m,
		modTime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		isDir:   (stat.Mode & unix.S_IFMT) == unix.S_IFDIR,
		sys:     sys,
	}
}
