package daemon

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// SecureReader defines the interface for reading files after enforcing security invariants.
// targetPath is interpreted relative to rootDir.
type SecureReader interface {
	ReadFile(rootDir, targetPath string) ([]byte, error)
}

// defaultSecureReader validates the path hierarchy, ownership, and permissions of
// agent-written files inside the Public Directory before reading them.
type defaultSecureReader struct {
	// openRoot is the seam used to construct the confined root filesystem. Production
	// wires this to openRootOS; tests substitute a closure to simulate openRoot
	// failures without touching the filesystem.
	openRoot func(path string) (rootFs, error)
}

func newDefaultSecureReader() *defaultSecureReader {
	return &defaultSecureReader{openRoot: openRootOS}
}

// ReadFile validates the path within rootDir and reads the contents of targetPath,
// which must be relative to rootDir.
//
// It ensures that:
//  1. rootDir itself is a root-owned 0700 directory (and not a symlink).
//  2. targetPath is a local path confined to rootDir; no component can escape the root
//     via ".." or symlinks.
//  3. Every directory along targetPath is root-owned with mode 0700.
//  4. The target file is root-owned with mode 0600.
func (r *defaultSecureReader) ReadFile(rootDir, targetPath string) ([]byte, error) {
	root, err := r.openRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not open root %q: %w", rootDir, err)
	}
	defer root.Close()

	// Validate the root directory itself: Lstat(".") sees the directory actually opened,
	// which is pinned to the same descriptor subsequent operations will target.
	if stat, err := root.Lstat("."); err != nil {
		return nil, fmt.Errorf("could not stat root %q: %v", rootDir, err)
	} else if err := defaultValidate(stat); err != nil {
		return nil, fmt.Errorf("refused %q: %v", rootDir, err)
	}

	// Open the file descriptor first and hold it open.
	targetFile, err := root.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %q: %w", filepath.Join(rootDir, targetPath), err)
	}
	defer targetFile.Close()

	// Validate each component from the root down to the target.
	segments := strings.Split(filepath.Clean(targetPath), string(filepath.Separator))
	current := ""
	for _, seg := range segments {
		current = filepath.Join(current, seg)
		stat, err := root.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("could not stat %q: %w", filepath.Join(rootDir, current), err)
		}
		if err := defaultValidate(stat); err != nil {
			return nil, fmt.Errorf("refused %q: %v", filepath.Join(rootDir, current), err)
		}
	}

	// Validate the open target file descriptor itself to ensure it points to a compliant inode
	// and was not substituted by an attacker prior to validation.
	targetStat, err := targetFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat %q: %v", filepath.Join(rootDir, targetPath), err)
	}
	if err := defaultValidate(targetStat); err != nil {
		return nil, fmt.Errorf("refused %q: %v", filepath.Join(rootDir, targetPath), err)
	}

	// Only then read the file contents.
	data, err := io.ReadAll(targetFile)
	if err != nil {
		return nil, fmt.Errorf("could not read %q: %v", filepath.Join(rootDir, targetPath), err)
	}

	return data, nil
}

// confinedFile represents an open file within rootFs that can report its own descriptor metadata.
type confinedFile interface {
	io.ReadCloser

	// Stat returns the file attributes of the open descriptor.
	Stat() (fileStat, error)
}

// rootFs defines the operations required by defaultSecureReader for reading files
// within a confined root directory without following symlinks.
//
// Paths passed to rootFs methods are interpreted relative to the root and may not
// escape it nor be symlinks.
//
// Why Lstat is load-bearing: the read walk calls Lstat on every component from the root
// to the target, including the root itself via Lstat("."). Lstat is the only stat flavor
// that returns the symlink itself rather than following it; replacing it with Stat would
// silently allow a symlink anywhere along targetPath to be resolved to its target,
// bypassing validateNode's symlink check and the per-component ownership/mode checks.
// The agent doesn't create symlinks inside its public dir, so no reason for us to accept symlinks.
type rootFs interface {
	io.Closer

	// Lstat stats a root-relative path without following symlinks.
	Lstat(name string) (fileStat, error)
	// Open opens a root-relative file for reading without following symlinks.
	Open(name string) (confinedFile, error)
}

// defaultValidate validates that a file or directory is strictly owned by root (UID 0, GID 0)
// with strict permissions (0700 for directories, 0600 for regular files), as mandated by the
// Secure Projection contract. Refusals report only the actual state observed.
var (
	expectedUID uint32
	expectedGID uint32
)

func defaultValidate(stat fileStat) error {
	if stat.UID != expectedUID || stat.GID != expectedGID {
		return fmt.Errorf("not strictly owned by root (uid %d, gid %d)", stat.UID, stat.GID)
	}

	if stat.Mode&modeSpecialBits != 0 {
		return errors.New("special permission bits (setuid/setgid/sticky) are not permitted")
	}

	perm := stat.Mode & modePermMask
	fileType := stat.Mode & modeTypeMask
	switch fileType {
	case modeDir:
		if perm != 0o700 {
			return fmt.Errorf("directory not strictly owned by root (mode 0%o)", perm)
		}
		return nil
	case modeReg:
		if perm != 0o600 {
			return fmt.Errorf("file not strictly owned by root (mode 0%o)", perm)
		}
		return nil
	case modeSymlink:
		return errors.New("symlinks are not permitted")
	default:
		return fmt.Errorf("irregular file type (mode 0%o)", stat.Mode)
	}
}

// Standard POSIX file type, mode, and permission constants.
const (
	modeTypeMask    uint32 = 0o170000
	modeDir         uint32 = 0o040000
	modeReg         uint32 = 0o100000
	modeSymlink     uint32 = 0o120000
	modeSpecialBits uint32 = 0o007000 // S_ISUID (04000) | S_ISGID (02000) | S_ISVTX (01000)
	modePermMask    uint32 = 0o000777
)

// fileStat holds the file attributes needed for secure validation.
type fileStat struct {
	Mode uint32
	UID  uint32
	GID  uint32
}
