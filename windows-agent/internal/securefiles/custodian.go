// Package securefiles implements a file system custodian component that enforces to all files and
// directories it creates under its root directory: all nodes have specific NT File Extended
// Attributes (EAs) stamped causing their projections via 9P inside WSL look as owned by root.
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
	"sync"

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
	// ErrNotOwned is returned when an operation requires an already-owned node,
	// but the source node lacks the custodian's watermark.
	ErrNotOwned = errors.New("node is not owned by custodian")
	// ErrRootReplaced is returned when the directory opened as the custodian's root is
	// not the one it created and stamped, meaning the path was redirected in between.
	ErrRootReplaced = errors.New("root directory was replaced between creation and open")

	// ErrDegraded is reported by CheckProjection when the sub-tree cannot be stamped at all.
	ErrDegraded = errors.New("sub-tree cannot be stamped")
	// ErrRemoteVolume is reported by CheckProjection when the sub-tree lives on a volume
	// this machine does not own. Stamping succeeds there and the attribute is stored
	// faithfully, but the projection does not honour it, so the guarantee does not hold.
	ErrRemoteVolume = errors.New("sub-tree is on a remote volume")
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
	return open(basePath, newPlatformSys)
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

// CheckProjection reports every condition under which the sub-tree keeps serving without
// the Secure Projection it promises (ADR 2.02), joining each that applies, or nil.
//
// It reports rather than logs because it cannot know where its findings belong: the agent
// opens its public directory before it has a log file to put inside it. The caller decides
// when it has somewhere durable to say this, and how loudly.
//
// It checks rather than recalls: classifying the volume queries the operating system,
// which blocks for seconds on an unreachable network path. Call it at startup, not on a
// path that serves requests.
func (c *Custodian) CheckProjection() error {
	var errs []error

	if cause := c.degradationCause(); cause != nil {
		errs = append(errs, fmt.Errorf("%w: %s cannot carry the ownership watermark, so what instances see is not projected as root-owned: %w",
			ErrDegraded, c.BasePath(), cause))
	}

	// Classify where the sub-tree really is, not where it was named: a directory in the
	// profile can be a link to a share, and then the name says "C:" while the bytes do not.
	where := c.basePath
	if resolved := c.resolvedBasePath(); resolved != "" {
		where = resolved
	}

	if remote, kind := remoteVolume(where); remote {
		// Name the resolved location too when it differs, or the report reads as a
		// contradiction: a path beginning "C:" is not obviously on a UNC path.
		location := kind
		if where != c.basePath {
			location = fmt.Sprintf("%s, resolving to %s", kind, where)
		}
		errs = append(errs, fmt.Errorf("%w: %s is on %s; the ownership stamp is stored there but the projection inside instances does not honour it, so nodes are exposed to unprivileged processes even though stamping succeeds", ErrRemoteVolume, c.basePath, location))
	}

	return errors.Join(errs...)
}

// BasePath returns the absolute path of the custodian's sub-tree root.
func (c *Custodian) BasePath() string {
	if c.relPath == "" {
		return c.basePath
	}
	return filepath.Join(c.basePath, c.relPath)
}

// Subdir returns a new custodian scoped to subDir inside this custodian's sub-tree.
func (c *Custodian) Subdir(subDir string) (*Custodian, error) {
	rel, err := c.resolve(subDir)
	if err != nil {
		return nil, err
	}

	// Create the sub-directory, stamped in the same syscall. A sub-tree root left by
	// an earlier run is adopted instead, and stamped in place: ADR 2.01 requires
	// first-level sub-tree roots to carry the stamp even when pre-existing, because it
	// is what revokes unprivileged creation and deletion inside them.
	err = c.sys.createNode(rel, true)
	if errors.Is(err, os.ErrExist) {
		err = c.sys.stampSubdir(rel)
	}
	if err != nil {
		return nil, mapEscape(err)
	}

	// Open a nested root at the sub-directory so the child custodian is itself
	// structurally contained and syscalls are rooted at its handle.
	subRoot, err := c.root.OpenRoot(rel)
	if err != nil {
		return nil, mapEscape(err)
	}

	// The child is derived from the parent's handle, never from its absolute path:
	// re-resolving the path here would walk components outside the parent's
	// containment, following any reparse point planted along the way.
	// The child shares the parent's degradation record and its syscall table: the record
	// because the loss of the watermark is a fact about the volume they both sit on, the
	// table so that what a test substitutes on a parent still holds for its sub-trees.
	subSys := newSubPlatformSys(c.sys)
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

// CreateMode selects what CreateFile does with a node that is already in place.
// The zero value replaces it, which is what most callers want: a node the custodian
// hands out should be one it created and stamped, not one it inherited.
type CreateMode int

const (
	// Replace discards any pre-existing node, so the returned file is a freshly
	// stamped, empty one.
	Replace CreateMode = iota
	// Append keeps what is already there and positions writes at the end, creating
	// and stamping the node only when it is absent. A caller that rotated a file away
	// and cannot tell whether the rotation succeeded needs this: replacing would
	// destroy the only remaining copy.
	Append
)

// CreateFile creates a file in the custodian sub-tree and returns it open for writing.
// Without a mode it replaces whatever is there; pass Append to add to it instead.
func (c *Custodian) CreateFile(name string, mode ...CreateMode) (*os.File, error) {
	targetRel, err := c.resolve(name)
	if err != nil {
		return nil, err
	}

	flags := os.O_WRONLY
	if len(mode) > 0 && mode[0] == Append {
		// Adopt what is there, and create only when nothing is.
		flags |= os.O_APPEND
		if _, err := c.root.Stat(targetRel); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, mapEscape(err)
			}
			if err := c.sys.createNode(targetRel, false); err != nil {
				return nil, mapEscape(err)
			}
		}
	} else {
		// Unlink to revoke any descriptor already open on the node, then create a fresh
		// one. Truncating would not do: the file object survives it, so a process that
		// opened the node while it was still unstamped would follow it into its replaced
		// life and read whatever is written next. Unlinking is the revocation; creating
		// is the cheap part.
		if err := c.root.Remove(targetRel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, mapEscape(err)
		}
		if err := c.sys.createNode(targetRel, false); err != nil {
			return nil, mapEscape(err)
		}
	}

	f, err := c.root.OpenFile(targetRel, flags, 0)
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

// Purge removes every unrecognised node and leftover temporary in the sub-tree, keeping
// what isAllowed accepts, and returns the relative names it removed.
//
// isAllowed is told whether the node is a directory because the listing already
// established it; a policy given only the name would have to open every entry. That answer
// is the one the listing saw, so a node changing type afterwards is judged on the older
// reading — harmless here, where a directory only ever means "not ours".
func (c *Custodian) Purge(isAllowed func(relPath string, isDir bool) bool) ([]string, error) {
	f, err := c.root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	var removed []string
	var failures []error

	for _, entry := range entries {
		name := entry.Name()

		if strings.HasPrefix(name, ".tmp-") || !isAllowed(name, entry.IsDir()) {
			if err := c.root.RemoveAll(name); err != nil {
				// One node that resists removal must not shield the rest of the sub-tree
				// from being purged, so the sweep continues and the failures are
				// reported together at the end.
				failures = append(failures, fmt.Errorf("failed to purge %q: %v", name, err))
				continue
			}
			removed = append(removed, name)
			log.Infof(context.Background(), "securefiles: purged unrecognised node or leftover temporary: %s", name)
		}
	}

	return removed, errors.Join(failures...)
}

// open builds a custodian over the platform layer newSys returns. Production always
// passes newPlatformSys; the parameter exists so that the tests in this package can
// supply a platform whose stamping fails from the outset, which is the one condition
// no test machine can produce on demand. It is unexported and has no exported caller,
// so a shipped binary offers no way to substitute the platform layer.
func open(basePath string, newSys func(string) (*platformSys, error)) (*Custodian, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %v", err)
	}

	// Ensure parent dir exists
	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, DirMode); err != nil {
		return nil, fmt.Errorf("failed to create parent directory %s: %v", parent, err)
	}

	sys, err := newSys(absPath)
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

	return c, nil
}

// degradation is the tree-wide record of what first proved that the filesystem cannot
// carry the watermark. The root and every sub-custodian share one: carrying extended
// attributes is a property of the volume, and no sub-tree can be on another volume,
// because the root refuses reparse points and a sub-tree root is opened from the parent's
// own handle rather than re-resolved by path. A sub-tree discovering the loss is the tree
// discovering it.
//
// A nil cause means healthy: degraded without a reason cannot be represented.
type degradation struct {
	mu    sync.Mutex
	first error
}

// note keeps the first cause and ignores the rest: later failures are the same fact
// about the volume, observed again somewhere else.
func (d *degradation) note(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.first == nil {
		d.first = err
	}
}

// cause returns what first prevented stamping, or nil while the sub-tree is healthy.
func (d *degradation) cause() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.first
}

// degradationCause returns what first prevented stamping anywhere in this custodian's
// tree, or nil while it is healthy.
func (c *Custodian) degradationCause() error {
	if c.sys == nil {
		return nil
	}
	return c.sys.deg.cause()
}

// resolvedBasePath returns the sub-tree root as the filesystem finally names it, or ""
// when no platform is attached to answer.
func (c *Custodian) resolvedBasePath() string {
	if c.sys == nil {
		return ""
	}
	return c.sys.resolvedBasePath()
}

// resolve validates name lexically and returns its cleaned form relative to this
// custodian's sub-tree. Only plain relative paths are dealt in: anything absolute,
// drive-qualified, or escaping via ".." with either separator is rejected here, so the
// platform layer never evaluates such a name — on Windows one reaching the NT syscalls
// would fail with an opaque status instead of the sentinel. Physical containment is
// enforced per operation by os.Root, whose resolution leaves no check-then-act gap.
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

func tempName(targetName string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	dir, base := filepath.Split(targetName)
	tmpBase := fmt.Sprintf(".tmp-%s-%s", base, hex.EncodeToString(b[:]))
	return filepath.Join(dir, tmpBase)
}
