// Package securefiles implements a file system custodian component that enforces to all files and
// directories it creates under its root directory: all nodes have specific NT File Extended
// Attributes (EAs) stampped causing their projections via 9P inside WSL look as owned by root.
package securefiles

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
)

// DirMode and FileMode are the permissions applied to directories and files managed by the custodian.
const (
	DirMode  fs.FileMode = 0700
	FileMode fs.FileMode = 0600
)

var (
	// ErrPathEscapes is returned when a requested relative path leaves the custodian's sub-tree.
	ErrPathEscapes = errors.New("path escapes sub-tree")
)

// Custodian scopes filesystem operations to a sub-tree and stamps nodes with their projected ownership.
// The whole sub-tree is held as an os.Root, so a name can never leave it: absolute paths, volume names,
// ".." escapes and symlinks whose target points outside are rejected by the standard library rather than
// by hand-written path checks. A sub-custodian owns a nested os.Root opened at its own directory, so it
// is structurally incapable of naming a sibling's subtree.
type Custodian struct {
	root     *os.Root
	basePath string
	relPath  string
	sys      *platformSys
}

// Open returns a custodian for the base path, creating and stamping it first if it does not exist.
// Pre-existing content is adopted: a consumer with data that must survive a
// restart (such as per-distro cloud-init files) can read it before rewriting its nodes.
func Open(basePath string) (*Custodian, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %v", err)
	}

	// Ensure parent dir exists
	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, DirMode); err != nil {
		return nil, fmt.Errorf("failed to create parent directory %s: %v", parent, err)
	}

	sys, err := newPlatformSys(absPath)
	if err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(absPath)
	if err != nil {
		sys.Close()
		return nil, fmt.Errorf("failed to open root for %s: %v", absPath, err)
	}
	if err := sys.setRoot(root); err != nil {
		root.Close()
		sys.Close()
		return nil, fmt.Errorf("failed to open root for %s: %v", absPath, err)
	}

	c := &Custodian{
		root:     root,
		basePath: absPath,
		relPath:  "",
		sys:      sys,
	}

	c.LogDegradedOnce()
	return c, nil
}

// Close releases any resources held by the custodian.
func (c *Custodian) Close() error {
	var errs []error
	if c.sys != nil {
		if err := c.sys.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.root != nil {
		if err := c.root.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// IsDegraded reports whether the custodian is operating in degraded mode.
func (c *Custodian) IsDegraded() bool {
	if c.sys != nil {
		return c.sys.isDegraded()
	}
	return false
}

// LogDegradedOnce logs an error once if the custodian is degraded.
func (c *Custodian) LogDegradedOnce() {
	if c.IsDegraded() {
		log.Errorf(context.Background(), "securefiles: underlying filesystem at %s does not support extended attributes; operating in degraded mode without secure projection", c.basePath)
	}
}

// BasePath returns the absolute path of the custodian's sub-tree root.
func (c *Custodian) BasePath() string {
	if c.relPath == "" {
		return c.basePath
	}
	return filepath.Join(c.basePath, c.relPath)
}

// resolve validates name lexically and returns its cleaned form relative to
// this custodian's sub-tree. The custodian only deals in plain relative paths:
// anything absolute, drive-qualified, or escaping via ".." (with either
// separator) is rejected here, so the platform layer never evaluates such
// names at all — on Windows an absolute path reaching the NT syscalls would
// fail with an opaque status instead of the sentinel. Physical containment
// (symlink escapes) is enforced per operation by the underlying os.Root, whose
// openat2-style resolution leaves no check-then-act gap.
func (c *Custodian) resolve(name string) (string, error) {
	if isBackslashDotDot(name) || isAnchored(name) {
		return "", ErrPathEscapes
	}
	rel := filepath.Clean(name)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathEscapes
	}
	return rel, nil
}

// isAnchored reports whether name is absolute or drive-qualified under either
// platform's rules: os.Root only enforces anchoring natively for the host
// platform, but the custodian contract is platform-deterministic.
func isAnchored(name string) bool {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return true
	}
	// Drive-qualified ("C:\foo") or drive-relative ("C:foo") Windows path.
	return len(name) >= 2 && name[1] == ':'
}

// mapEscape translates the os.Root containment error into the public sentinel.
func mapEscape(err error) error {
	if isEscapeError(err) {
		return ErrPathEscapes
	}
	return err
}

// isBackslashDotDot reports whether name contains a ".." component written with
// the Windows separator, which os.Root only treats as an escape on Windows. On
// other platforms "..\x" is an ordinary literal filename, so it is rejected
// explicitly to keep the escape contract uniform.
func isBackslashDotDot(name string) bool {
	for _, part := range strings.Split(name, `\`) {
		if part == ".." {
			return true
		}
	}
	return false
}

// isEscapeError reports whether err is the os.Root path-escaping error. The
// standard library does not export a sentinel for it, so it is recognised by
// its fixed message, wrapped in an *os.PathError or an *os.LinkError
// (rename operations return the latter).
func isEscapeError(err error) bool {
	const escapeMsg = "path escapes from parent"
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil && pathErr.Err.Error() == escapeMsg {
		return true
	}
	var linkErr *os.LinkError
	return errors.As(err, &linkErr) && linkErr.Err != nil && linkErr.Err.Error() == escapeMsg
}

// Subdir returns a new custodian scoped to subDir inside this custodian's sub-tree.
func (c *Custodian) Subdir(subDir string) (*Custodian, error) {
	rel, err := c.resolve(subDir)
	if err != nil {
		return nil, err
	}

	// Ensure the sub-directory exists (create if missing)
	if err := c.sys.createNode(rel, true); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, mapEscape(err)
	}

	// Open a nested root at the sub-directory so the child custodian is itself
	// structurally contained and syscalls are rooted at its handle.
	subRoot, err := c.root.OpenRoot(rel)
	if err != nil {
		return nil, mapEscape(err)
	}
	subSys, err := newPlatformSys(filepath.Join(c.BasePath(), rel))
	if err != nil {
		subRoot.Close()
		return nil, err
	}
	if err := subSys.setRoot(subRoot); err != nil {
		subRoot.Close()
		subSys.Close()
		return nil, err
	}

	return &Custodian{
		root:     subRoot,
		basePath: c.basePath,
		relPath:  filepath.Join(c.relPath, subDir),
		sys:      subSys,
	}, nil
}

func tempName(targetName string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	dir, base := filepath.Split(targetName)
	tmpBase := fmt.Sprintf(".tmp-%s-%s", base, hex.EncodeToString(b[:]))
	return filepath.Join(dir, tmpBase)
}

// WriteFile atomically writes data to a file relative to the custodian's sub-tree.
func (c *Custodian) WriteFile(name string, data []byte) error {
	targetRel, err := c.resolve(name)
	if err != nil {
		return err
	}

	tmpRel := tempName(targetRel)

	if err := c.sys.createNode(tmpRel, false); err != nil {
		return mapEscape(err)
	}
	defer func() {
		_ = c.root.Remove(tmpRel)
	}()

	// The temp node exists (createNode made it), so WriteFile only truncates and
	// writes: the mode and the stamp from createNode are preserved.
	if err := c.root.WriteFile(tmpRel, data, FileMode); err != nil {
		return mapEscape(err)
	}

	return mapEscape(c.sys.renameNode(tmpRel, targetRel))
}

// IsOwned reports whether the named node was created by a custodian (carries the
// platform watermark) and is unaltered since.
func (c *Custodian) IsOwned(name string) (bool, error) {
	rel, err := c.resolve(name)
	if err != nil {
		return false, err
	}
	owned, err := c.sys.isOwned(rel)
	return owned, mapEscape(err)
}

// CreateFile creates or replaces a file in the custodian sub-tree, returning it
// open for writing. The semantics are fresh-start: any pre-existing node is
// discarded and the returned file is a newly stamped, empty node. This is what
// log rotation wants: the new log must never read as an adopted previous file.
func (c *Custodian) CreateFile(name string) (*os.File, error) {
	targetRel, err := c.resolve(name)
	if err != nil {
		return nil, err
	}

	// Fresh-start: discard any pre-existing node so the returned file is a
	// freshly stamped node rather than an adopted one.
	if _, err := c.root.Stat(targetRel); err == nil {
		if err := c.root.Remove(targetRel); err != nil {
			return nil, mapEscape(err)
		}
	}

	if err := c.sys.createNode(targetRel, false); err != nil {
		return nil, mapEscape(err)
	}

	f, err := c.root.OpenFile(targetRel, os.O_WRONLY, 0)
	if err != nil {
		return nil, mapEscape(err)
	}
	return f, nil
}

// Remove deletes the named node relative to the custodian's sub-tree.
func (c *Custodian) Remove(name string) error {
	rel, err := c.resolve(name)
	if err != nil {
		return err
	}
	return mapEscape(c.root.Remove(rel))
}

// RemoveAll deletes the named node and any subtree rooted at it.
func (c *Custodian) RemoveAll(name string) error {
	rel, err := c.resolve(name)
	if err != nil {
		return err
	}
	return mapEscape(c.root.RemoveAll(rel))
}

// ReadDir lists the named directory relative to the custodian's sub-tree.
func (c *Custodian) ReadDir(name string) ([]os.DirEntry, error) {
	rel, err := c.resolve(name)
	if err != nil {
		return nil, err
	}
	f, err := c.root.Open(rel)
	if err != nil {
		return nil, mapEscape(err)
	}
	defer f.Close()
	return f.ReadDir(-1)
}

// Rename atomically replaces newName with oldName, both relative to the
// custodian's sub-tree.
func (c *Custodian) Rename(oldName, newName string) error {
	oldRel, err := c.resolve(oldName)
	if err != nil {
		return err
	}
	newRel, err := c.resolve(newName)
	if err != nil {
		return err
	}
	return mapEscape(c.sys.renameNode(oldRel, newRel))
}

// Purge removes all unrecognised nodes and leftover temporaries in the custodian's sub-tree
// based on the caller-supplied policy function isAllowed. Returns the list of removed relative names.
func (c *Custodian) Purge(isAllowed func(relPath string) bool) ([]string, error) {
	f, err := c.root.Open(".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		name := entry.Name()

		if strings.HasPrefix(name, ".tmp-") || !isAllowed(name) {
			if err := c.root.RemoveAll(name); err == nil {
				removed = append(removed, name)
				log.Infof(context.Background(), "securefiles: purged unrecognised node or leftover temporary: %s", name)
			}
		}
	}

	return removed, nil
}

// SetMockDegraded forces the custodian into degraded mode for testing.
func (c *Custodian) SetMockDegraded(degraded bool) {
	if c.sys != nil {
		c.sys.setMockDegraded(degraded)
	}
}

// SetMockOwned forces the ownership predicate to return owned for testing. A
// nil value restores the per-platform behaviour.
func (c *Custodian) SetMockOwned(owned *bool) {
	if c.sys != nil {
		c.sys.setMockOwned(owned)
	}
}
