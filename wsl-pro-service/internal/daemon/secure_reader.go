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

// rootFs exists for the same reason as the surrounding seam, plus a Go-language
// constraint: *os.Root.Open returns *os.File, but this interface narrows the return to
// io.ReadCloser so the test mock can hand back an io.NopCloser(bytes.NewReader(...))
// without spinning up a real temporary file. Because Go interface satisfaction requires
// identical method signatures, *os.Root does not satisfy rootFs as written, which is
// why osRootAdapter is needed to bridge it. Removing the io.ReadCloser narrowing (or
// the adapter that implements it) would force the mock into real temp files; do not do
// either without also redesigning the table tests in secure_reader_test.go.
//
// Why Lstat is load-bearing: the read walk calls Lstat on every component from the root
// to the target, including the root itself via Lstat("."). Lstat is the only stat flavor
// that returns the symlink itself rather than following it; replacing it with Stat would
// silently allow a symlink anywhere along targetPath to be resolved to its target,
// bypassing validateNode's symlink check and the per-component ownership/mode checks.
// The agent doesn't create symlinks inside its public dir, so no reason for us to accept symlinks.
//
// Paths passed to rootFs methods are interpreted relative to the root and may not
// escape it; that confinement is provided by *os.Root, not by this interface.
type rootFs interface {
	io.Closer

	// Lstat stats a root-relative path without following symlinks.
	Lstat(name string) (fs.FileInfo, error)
	// Open opens a root-relative file for reading.
	Open(name string) (io.ReadCloser, error)
}

// openRootOS is the production openRoot seam used by defaultSecureReader. It pins the
// named directory via os.OpenRoot (kernel-enforced confinement against ".." and
// symlink escapes) and adapts *os.Root to rootFs so the inner interface can hand back
// io.ReadCloser from Open. Tests substitute their own openRoot to simulate OpenRoot
// failures without touching the filesystem; see secure_reader_test.go.
func openRootOS(path string) (rootFs, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return osRootAdapter{root}, nil
}

// osRootAdapter bridges *os.Root (production) and rootFs (interface). The narrow
// io.ReadCloser return on rootFs.Open is the reason this wrapper exists; see rootFs
// for the full rationale. Every method is a one-line forward to *os.Root.
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
	// openRoot is the seam used to construct the confined root filesystem. Production
	// wires this to openRootOS; tests substitute a closure to simulate OpenRoot
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
//  2. targetPath is a local path confined to rootDir; os.Root guarantees that no
//     component can escape the root via ".." or symlinks, even under concurrent renames.
//  3. Every directory along targetPath is root-owned with mode 0700.
//  4. The target file is root-owned with mode 0600.
func (r *defaultSecureReader) ReadFile(rootDir, targetPath string) ([]byte, error) {
	// os.OpenRoot follows symlinks to directories, so a symlink root whose target is
	// itself a directory would slip past root.Lstat(".") below. Reject symlinks at
	// the absolute path before opening.
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

	// Validate the root directory itself: Lstat(".") sees the directory os.Root opened,
	// which is pinned to the same inode subsequent operations will target. This closes
	// the small TOCTOU window between an absolute-path Lstat and OpenRoot.
	if fi, err := root.Lstat("."); err != nil {
		return nil, fmt.Errorf("could not stat root %q: %w", rootDir, err)
	} else if err := validateNode(rootDir, fi); err != nil {
		return nil, err
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
