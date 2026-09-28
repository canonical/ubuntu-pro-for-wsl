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

// osFs abstracts OS-level filesystem interactions for defaultSecureReader.
type osFs interface {
	// Lstat stats an absolute path without following symlinks (used for the root itself).
	Lstat(path string) (fs.FileInfo, error)
	// OpenRoot opens the named directory as a confined filesystem root.
	OpenRoot(path string) (rootFs, error)
}

// rootFs abstracts path operations confined to a root directory.
// Paths passed to its methods are interpreted relative to the root and may not escape it.
type rootFs interface {
	io.Closer

	// Lstat stats a root-relative path without following symlinks.
	Lstat(name string) (fs.FileInfo, error)
	// Open opens a root-relative file for reading.
	Open(name string) (io.ReadCloser, error)
}

// realOSFs implements osFs using the os package, relying on os.Root to confine all
// root-relative path resolution to the root directory (no symlink or ".." escapes).
type realOSFs struct{}

func (realOSFs) Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(path)
}

func (realOSFs) OpenRoot(path string) (rootFs, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return osRootAdapter{root}, nil
}

// osRootAdapter adapts *os.Root to the rootFs interface, narrowing Open's return
// type from *os.File to io.ReadCloser.
type osRootAdapter struct {
	root *os.Root
}

func (a osRootAdapter) Lstat(name string) (fs.FileInfo, error) {
	return a.root.Lstat(name)
}

func (a osRootAdapter) Open(name string) (io.ReadCloser, error) {
	return a.root.Open(name)
}

func (a osRootAdapter) Close() error {
	return a.root.Close()
}

// defaultSecureReader validates the path hierarchy, ownership, and permissions of
// agent-written files inside the Public Directory before reading them.
type defaultSecureReader struct {
	fs osFs
}

func newDefaultSecureReader() *defaultSecureReader {
	return &defaultSecureReader{fs: realOSFs{}}
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
//  2. targetPath is a local path confined to rootDir; os.Root guarantees that no
//     component can escape the root via ".." or symlinks, even under concurrent renames.
//  3. Every directory along targetPath is root-owned with mode 0700.
//  4. The target file is root-owned with mode 0600.
func (r *defaultSecureReader) ReadFile(rootDir, targetPath string) ([]byte, error) {
	// Validate the root directory itself.
	fi, err := r.fs.Lstat(rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not stat %q: %w", rootDir, err)
	}
	if err := validateNode(rootDir, fi); err != nil {
		return nil, err
	}

	root, err := r.fs.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not open root %q: %w", rootDir, err)
	}
	defer root.Close()

	// The target must be a local path inside the root.
	if !filepath.IsLocal(targetPath) {
		return nil, fmt.Errorf("refused %q: path escapes root %q", targetPath, rootDir)
	}

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

	f, err := root.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %q: %w", filepath.Join(rootDir, targetPath), err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("could not read %q: %w", filepath.Join(rootDir, targetPath), err)
	}

	return data, nil
}
