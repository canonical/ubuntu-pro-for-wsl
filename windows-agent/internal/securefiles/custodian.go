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

	// ErrNoWatermarkSupport reports a filesystem that cannot carry the ownership
	// watermark. Nothing on it can be projected as root-owned inside an instance, so the
	// custodian refuses rather than publishing the agent's credentials unprotected.
	ErrNoWatermarkSupport = errors.New("filesystem cannot carry the ownership watermark")
	// ErrRemoteVolume is reported by CheckProjection when the sub-tree lives on a volume
	// this machine does not own. Stamping succeeds there and the attribute is stored
	// faithfully; what cannot be verified from here is what an instance makes of it,
	// which is why this is reported rather than refused like ErrNoWatermarkSupport.
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

// CheckProjection reports every condition under which the sub-tree keeps serving without
// the Secure Projection it promises, joining each that applies, or nil. A sub-tree that
// cannot be stamped never gets this far: Open refuses it (ADR 2.02). What remains is the
// condition no check on this machine can refuse, because the stamp is stored faithfully
// and only the projection inside an instance ignores it.
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
		errs = append(errs, fmt.Errorf("%w: %s is on %s; the ownership stamp is stored there faithfully, but the projection may not honour it", ErrRemoteVolume, c.basePath, location))
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
	_, err = c.sys.createNode(rel, true)
	if errors.Is(err, os.ErrExist) || errors.Is(err, ErrPathEscapes) || isEscapeError(err) {
		// The create cannot tell what stands at the name: a link answers a name
		// collision for some kinds and an escape refusal for others, and a real
		// directory answers a collision too. It is recognized by attribute instead
		// (ADR 2.03): a reparse point standing at a sub-tree root is never adopted
		// data — nothing unprivileged can write inside a stamped tree (ADR 2.01), so
		// a link there was planted before the tree was stamped, and following it
		// reaches only what the planter chose — so it is removed and the directory
		// created in its place. A real directory is adopted and stamped in place.
		// (Where the tree root itself is a link, refusing stays the answer: Open
		// runs before there is a stamped tree to vouch for the neighborhood.)
		// What the check cannot answer, and a link that cannot be removed, refuse
		// the sub-tree: the nested root below must never be opened through
		// something unchecked, so nothing is stamped before the attribute answers.
		var reparse bool
		reparse, err = c.sys.isReparsePoint(rel)
		if err == nil {
			if reparse {
				err = c.root.Remove(rel)
				if err == nil {
					_, err = c.sys.createNode(rel, true)
				} else {
					err = fmt.Errorf("could not remove the link at %s: %w", rel, err)
				}
			} else {
				err = c.sys.stampSubdir(rel)
			}
		}
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
	// The child inherits the parent's syscall table as it stands at the handout: a seam
	// installed on the parent before this call governs the sub-tree too; one installed
	// after does not reach an already-handed-out child.
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

// betweenCreateAndWrite is a seam internal tests use to act inside the window
// between the temporary's creation and the write that fills it, where a tamperer
// would act. It is nil in production.
var betweenCreateAndWrite func()

// WriteFile atomically writes data to a file relative to the custodian's sub-tree.
// The bytes travel through the descriptor the temporary was created with, and the
// publish verifies that the temporary's name still holds that node: no second name
// resolution sits between the stamp and either the write or the publish, so a name
// swapped in the window cannot redirect either into another stamped node.
func (c *Custodian) WriteFile(name string, data []byte) error {
	targetRel, err := c.resolve(name)
	if err != nil {
		return err
	}

	tmpRel := tempName(targetRel)

	tmp, err := c.sys.createNode(tmpRel, false)
	if err != nil {
		return mapEscape(err)
	}
	// The temporary's handle stays open until after the publish, so the node cannot
	// be reclaimed while the write is in flight. It does not pin the name: the handle
	// shares deletion, so a name planted or swapped in this window is answered by the
	// checks below, not by the handle.
	defer func() {
		_ = tmp.Close()
		_ = c.root.Remove(tmpRel)
	}()

	if betweenCreateAndWrite != nil {
		betweenCreateAndWrite()
	}

	// The write goes through the descriptor createNode returned, never through the
	// temporary's name. The fresh node is empty, so there is nothing to truncate, and
	// its mode and stamp are already in place.
	if _, err := tmp.Write(data); err != nil {
		return mapEscape(err)
	}

	return mapEscape(c.sys.renameNode(tmpRel, targetRel, tmp))
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

// CreateFile creates a file in the custodian sub-tree and returns it open for writing,
// replacing whatever node is there. The returned descriptor is the very node the
// custodian created and stamped: there is no second name resolution between the stamp
// and the caller's writes, so ownership cannot be swapped out from under the descriptor.
// A node another process still holds open is refused rather than truncated, so its
// descriptor can never follow the node into its replaced life.
//
// A caller that rotated a file away and cannot tell whether the rotation succeeded
// needs AppendFile instead: replacing would destroy the only remaining copy.
func (c *Custodian) CreateFile(name string) (*os.File, error) {
	targetRel, err := c.resolve(name)
	if err != nil {
		return nil, err
	}

	// Unlink to revoke any descriptor already open on the node, then create a fresh
	// one. Truncating would not do: the file object survives it, so a process that
	// opened the node while it was still unstamped would follow it into its replaced
	// life and read whatever is written next. Unlinking is the revocation; creating
	// is the cheap part. FILE_SUPERSEDE would collapse the two steps into one
	// syscall, but a holder sharing delete follows the node into its replaced life
	// and reads the replacement's writes, which is what unlinking prevents. Measured
	// against a local volume: a supersede succeeds against a holder opened with
	// FILE_SHARE_DELETE, and that holder then reads the new node's content; a holder
	// without delete sharing is refused, as our own replacement is.
	if err := c.root.Remove(targetRel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, mapEscape(err)
	}
	return c.sys.createNode(targetRel, false)
}

// AppendFile opens the named file for writing at its end, creating and stamping it only
// when it is absent. A node already in place is adopted as it is, whatever owns it:
// this mode exists for a caller that rotated a file away and cannot tell whether the
// rotation succeeded, where the file left behind may be the only remaining copy. The
// adoption policy belongs to the caller: one that must never write into a node it does
// not own checks IsOwned before appending, and replaces instead.
func (c *Custodian) AppendFile(name string) (*os.File, error) {
	targetRel, err := c.resolve(name)
	if err != nil {
		return nil, err
	}

	_, statErr := c.root.Stat(targetRel)
	switch {
	case errors.Is(statErr, fs.ErrNotExist):
		return c.sys.createNode(targetRel, false)
	case statErr != nil:
		return nil, mapEscape(statErr)
	}

	f, err := c.root.OpenFile(targetRel, os.O_WRONLY|os.O_APPEND, 0)
	return f, mapEscape(err)
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
	return mapEscape(c.sys.renameNode(oldRel, newRel, nil))
}

// PurgeAll removes every node in the sub-tree, whatever it is. It is for sub-trees
// whose contents are wholly the agent's to discard: an ephemeral credential store
// regenerated on every start, a cache. Where contents can still be data someone else
// needs, the caller decides what to keep and uses Purge with a predicate instead, and
// inspects what it removed: PurgeAll's caller has no use for the names, so they are
// only logged.
//
// The sub-tree root itself is never removed: the custodian keeps working from it, and
// it is the one node whose removal no caller of this method can mean.
func (c *Custodian) PurgeAll() error {
	f, err := c.root.Open(".")
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return err
	}

	var failures []error
	for _, entry := range entries {
		name := entry.Name()
		if err := c.root.RemoveAll(name); err != nil {
			// One node that resists removal must not shield the rest of the sub-tree
			// from being purged, so the sweep continues and the failures are reported
			// together at the end.
			failures = append(failures, fmt.Errorf("failed to purge %q: %v", name, err))
			continue
		}
		log.Infof(context.Background(), "securefiles: purged node: %s", name)
	}

	return errors.Join(failures...)
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
			// The names travel back to the caller, which knows what each one was and
			// logs the removal at its own severity: a bare list from here would log
			// every removal twice, once blind.
			removed = append(removed, name)
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
