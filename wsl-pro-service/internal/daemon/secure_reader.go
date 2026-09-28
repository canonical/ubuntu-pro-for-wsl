package daemon

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SecureReader defines the interface for reading files after enforcing security invariants.
// targetPath is interpreted relative to rootDir.
type SecureReader interface {
	ReadFile(rootDir, targetPath string) ([]byte, error)
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
	Lstat(name string) (fs.FileInfo, error)
	// Open opens a root-relative file for reading without following symlinks.
	Open(name string) (io.ReadCloser, error)
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

// defaultValidate validates that a file or directory is strictly owned by root (UID 0, GID 0)
// with strict permissions (0700 for directories, 0600 for regular files), as mandated by the
// Secure Projection contract. Refusals report only the actual state observed.
func defaultValidate(path string, info fs.FileInfo) error {
	uid, gid, err := ownershipOf(info)
	if err != nil {
		return fmt.Errorf("could not obtain ownership metadata for %q: %w", path, err)
	}

	if uid != 0 || gid != 0 {
		return fmt.Errorf("refused %q: not strictly owned by root (uid %d, gid %d)", path, uid, gid)
	}

	perm := info.Mode().Perm()
	if info.IsDir() {
		if perm != 0700 {
			return fmt.Errorf("refused directory %q: not strictly owned by root (mode 0%o)", path, perm)
		}
		return nil
	}

	if perm != 0600 {
		return fmt.Errorf("refused file %q: not strictly owned by root (mode 0%o)", path, perm)
	}

	return nil
}

func validateNode(path string, fi fs.FileInfo) error {
	// Reject symlinks unconditionally.
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("refused %q: symlinks are not permitted", path)
	}

	// Ensure regular file or directory.
	if !fi.IsDir() && !fi.Mode().IsRegular() {
		return fmt.Errorf("refused %q: irregular file type (mode %v)", path, fi.Mode())
	}

	return defaultValidate(path, fi)
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
	if fi, err := os.Lstat(rootDir); err != nil {
		return nil, fmt.Errorf("could not stat %q: %w", rootDir, err)
	} else if fi.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("refused %q: root is a symlink", rootDir)
	}

	root, err := r.openRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not open root %q: %w", rootDir, err)
	}
	defer root.Close()

	// Validate the root directory itself: Lstat(".") sees the directory actually opened,
	// which is pinned to the same descriptor subsequent operations will target.
	if fi, err := root.Lstat("."); err != nil {
		return nil, fmt.Errorf("could not stat root %q: %w", rootDir, err)
	} else if err := validateNode(rootDir, fi); err != nil {
		return nil, err
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
		fi, err := root.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("could not stat %q: %w", filepath.Join(rootDir, current), err)
		}
		if err := validateNode(filepath.Join(rootDir, current), fi); err != nil {
			return nil, err
		}
	}

	// Only then read the file contents.
	data, err := io.ReadAll(targetFile)
	if err != nil {
		return nil, fmt.Errorf("could not read %q: %w", filepath.Join(rootDir, targetPath), err)
	}

	return data, nil
}
