package securefiles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// doubleSys is the Linux platformSys: the contract double for the Windows
// implementation (sys_windows.go). No binary the project ships runs it — the
// agent is packaged for Windows only, and this file exists so the package and
// every consumer of it compiles and tests on Linux, the platform CI builds
// besides Windows.
//
// Its obligation is the custodian contract — what custodian.go and the
// cross-platform tests can observe — not the NT mechanisms behind it. The
// corner cases only those mechanisms deliver, none of them reproducible off
// Windows, are pinned by the Windows-only tests against the real filesystem:
// the share-mode root veto, stamping in the creating syscall, POSIX rename
// classes and their fallback, reparse kinds beyond symlinks.
type doubleSys struct {
	// stamps is the stamping surface: the xattr syscalls in production, a
	// configurable stand-in under test. A field of the layer rather than a
	// package-level variable so no process-wide switch exists that could turn
	// stamping off for every custodian at once; set once at construction
	// and never reassigned by production.
	stamps stampCalls

	// basePath is the path the layer was constructed at, recorded so
	// finalPath can answer it.
	//
	// rootFD is the root's own descriptor, dup'd out of the os.Root the layer
	// is attached to: os.Root cannot be asked for its handle, and every
	// openat2 resolution needs one to be issued against — the categorical
	// no-symlink open os.Root itself cannot perform, because it resolves an
	// in-tree link instead of refusing one. Held for the custodian's
	// lifetime, closed in release. release may run while served operations
	// still read the descriptor, so mu guards the pair: an in-flight
	// operation drains before the close, and every operation after answers
	// the closed descriptor instead of racing it. The lock is held through
	// the openat2 calls themselves — an atomic would order the field's
	// reads, but the hazard is the descriptor's lifetime: closed before an
	// operation uses it, its number is back in the allocator's pool, and
	// the next open in the process, whoever's it is, could hand openat2 a
	// foreign directory as the containment root. The Windows layer guards
	// its root handle the same way, for the same reason.
	mu       sync.Mutex
	rootFD   int
	basePath string
}

// stampCalls is the syscall surface the double's stamping path uses, wired by
// newDoubleSysWith so the tests in this package can stand in for a filesystem
// this machine is not, exactly as the Windows tests stand in for conditions
// only that filesystem can refuse.
type stampCalls struct {
	set  func(fd int, data []byte) error
	get  func(fd int, buf []byte) (int, error)
	list func(fd int) error
}

// realStampCalls returns the production implementation: the xattr syscalls
// themselves. User xattrs need no privileges for files the process owns,
// which matches every test environment, and they are the only persistent mark
// a plain filesystem offers that plain file content cannot imitate: ownership
// must survive the custodian, because the agent's log rotation renames a node
// the previous run stamped.
func realStampCalls() stampCalls {
	return stampCalls{
		set:  func(fd int, data []byte) error { return unix.Fsetxattr(fd, stampXattr, data, 0) },
		get:  func(fd int, buf []byte) (int, error) { return unix.Fgetxattr(fd, stampXattr, buf) },
		list: func(fd int) error { _, err := unix.Flistxattr(fd, nil); return err },
	}
}

// stampXattr is the user-namespace extended attribute the double stamps
// every node it creates with: the same stamp values the Windows layer writes
// as NT extended attributes, carried by the only channel a plain filesystem
// offers off Windows. No shipped binary runs the double, so the name carries
// no compatibility weight; the internal test that plants raw stamps uses
// this constant, not its own copy.
const stampXattr = "user.com.canonical.up4w"

// newPlatformSys returns the double for the path open() was handed. The
// double establishes nothing here: establishRoot does, once open() has created
// the parent directories the ordinary way.
func newPlatformSys(basePath string) platformSys {
	return newDoubleSysWith(basePath, realStampCalls())
}

// newDoubleSysWith is newPlatformSys with the stamping surface supplied
// rather than assumed.
func newDoubleSysWith(basePath string, stamps stampCalls) platformSys {
	return &doubleSys{basePath: basePath, stamps: stamps, rootFD: -1}
}

// sub returns the layer for a sub-custodian: the same stamping surface as the
// parent's, as it stands at the handout, and no path of its own to resolve —
// a sub-tree root is reached through the parent's containment.
func (s *doubleSys) sub() platformSys {
	return &doubleSys{basePath: s.basePath, stamps: s.stamps, rootFD: -1}
}

// establishRoot creates or adopts the tree root, stamped either way, and
// answers the identity it established so open() can tie its second resolution
// to this one. The leaf open carries O_NOFOLLOW, so a link standing at the
// root name is refused however it points — the categorical verdict the
// Windows layer answers through OBJ_DONT_REPARSE — rather than followed to a
// target the parent directory cannot vouch for: open() runs before there is a
// stamped tree to do that, so a link there is never adopted data. os.Root
// cannot carry the flag for us: it follows an in-tree link whatever the open
// asked for. POSIX
// cannot refuse a rename because a descriptor is open, so the double holds no
// root: the runtime root-swap window stays closed by instance-side validation
// instead (ADR 2.01).
func (s *doubleSys) establishRoot() (id fileIdentity, known bool, err error) {
	dir := filepath.Dir(s.basePath)
	leaf := filepath.Base(s.basePath)
	if err := os.Mkdir(filepath.Join(dir, leaf), DirMode); err != nil && !errors.Is(err, os.ErrExist) {
		return fileIdentity{}, false, err
	}

	// The leaf open is issued directly rather than through an os.Root: Root
	// opens every leaf with O_NOFOLLOW and then follows the link anyway when
	// its target stays inside the tree, so the flag cannot reach the kernel
	// through it. The direct openat keeps the refusal categorical, as
	// OBJ_DONT_REPARSE does on Windows. O_NOFOLLOW without O_DIRECTORY, so the
	// link answers ELOOP and a non-directory standing at the name keeps its
	// own verdict below.
	parentFD, err := unix.Open(dir, os.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fileIdentity{}, false, fmt.Errorf("could not open the parent of the tree root %s: %v", s.basePath, err)
	}
	defer func() { _ = unix.Close(parentFD) }()

	fd, err := unix.Openat(parentFD, leaf, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return fileIdentity{}, false, fmt.Errorf("the tree root %s is a link: %w", s.basePath, ErrPathEscapes)
		}
		return fileIdentity{}, false, fmt.Errorf("could not establish the tree root %s: %v", s.basePath, err)
	}
	f := os.NewFile(uintptr(fd), s.basePath) //#nosec G115 // a file descriptor is a small non-negative int; int->uintptr only widens.
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return fileIdentity{}, false, err
	}
	if !st.IsDir() {
		return fileIdentity{}, false, fmt.Errorf("the tree root %s is not a directory", s.basePath)
	}

	// Refuse a tree the double cannot stamp, rather than serving it unstamped
	// (ADR 2.02). The probe lists attributes on the root, a weaker question
	// than the stamping it predicts, so a filesystem that passes it and
	// refuses the stamp is answered by createStamped too. An unexplained
	// probe failure is no evidence that stamping works either, so this fails
	// closed on it.
	if err := s.stamps.list(int(f.Fd())); err != nil { //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
		if isXattrUnsupported(err) {
			return fileIdentity{}, false, fmt.Errorf("%w: %v", ErrNoStampSupport, err)
		}
		return fileIdentity{}, false, fmt.Errorf("could not establish stamp support: %v", err)
	}

	id, identified := identityOf(st)
	return id, identified, nil
}

// identity answers the node's dev and inode: what distinguishes the node the
// custodian established from whatever its path resolves to a moment later. An
// identity that cannot be read is not an identity, so a failure answers none
// rather than guessing.
func (s *doubleSys) identity(f *os.File) (fileIdentity, bool) {
	st, err := f.Stat()
	if err != nil {
		return fileIdentity{}, false
	}
	return identityOf(st)
}

// attach anchors the double's operations on the root the custodian opened and
// verified, opening its own descriptor beside the os.Root's (see rootFD).
// Construction-time only: this runs while the custodian is still being built,
// before it is reachable from any other goroutine.
func (s *doubleSys) attach(root *os.Root) error {
	// Open the root through the os.Root itself: "." resolves to the tree
	// root, and the returned descriptor is the anchor every openat2
	// resolution is issued against.
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	fd, err := unix.Dup(int(f.Fd())) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
	if err != nil {
		return err
	}
	if err := unix.SetNonblock(fd, false); err != nil {
		_ = unix.Close(fd)
		return err
	}
	s.rootFD = fd
	return nil
}

// release releases everything the double holds: the root descriptor beside
// the os.Root, which the custodian closes itself. Served operations read the
// descriptor and Close writes it, so the mutex guards the pair: an in-flight
// operation drains before the descriptor closes, and every operation after
// answers the closed descriptor instead of racing the close.
func (s *doubleSys) release() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rootFD >= 0 {
		_ = unix.Close(s.rootFD)
		s.rootFD = -1
	}
	return nil
}

// identityOf reads a fileIdentity off an os.FileInfo: the device number into
// the volume word, the inode into the low half of the file id. It answers
// usable=false when the platform supplies no Stat_t at all.
func identityOf(fi os.FileInfo) (fileIdentity, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}, false
	}
	id := fileIdentity{volume: st.Dev}
	binary.LittleEndian.PutUint64(id.file[:8], st.Ino)
	return id, id.file != [16]byte{}
}

// createStamped creates rel under the tree root, stamped as the custodian's
// own. POSIX cannot stamp in the creating syscall, so the stamp follows the
// open as closely as the platform allows: the fresh node is empty, so nothing
// unstamped is ever published through it, and a failure removes the node
// rather than leaving an unstamped one behind. A name collision answers
// os.ErrExist, and the adoption decision is the caller's (Subdir), not the
// create's.
func (s *doubleSys) createStamped(rel string, dir bool) (*os.File, error) {
	// Held through the openat2 below for the same reason as openNode's: the
	// descriptor it resolves against must outlive the syscall.
	s.mu.Lock()
	defer s.mu.Unlock()

	if dir {
		// Mkdirat on a pinned parent: the directory is created at the leaf of
		// a resolution that refused every link, the way the Windows create
		// does through OBJ_DONT_REPARSE. A link standing at the leaf answers
		// EEXIST — the adoption offer the caller decides about — instead of
		// being resolved through.
		pfd, leaf, err := s.openParent(rel)
		if err != nil {
			return nil, err
		}
		defer func() { _ = unix.Close(pfd) }()
		if err := unix.Mkdirat(pfd, leaf, uint32(DirMode)); err != nil { //#nosec G115 // DirMode is the small non-negative permission constant; the conversion cannot overflow.
			return nil, classifyOpenErr(err)
		}
		// The one place ADR 2.01 allows stamping in place — adoption — is the
		// caller's decision, answered by ErrExist above; a fresh directory
		// needs no stamp of its own on the double.
		return nil, nil
	}

	// The file create resolves the whole name under the same flags the open
	// uses: a link at any component refuses, and O_EXCL turns a leaf that
	// already exists — link or not — into EEXIST rather than a write to it.
	fd, err := unix.Openat2(s.rootFD, rel, &unix.OpenHow{
		Flags:   uint64(os.O_CREATE | os.O_EXCL | os.O_WRONLY | unix.O_CLOEXEC), //#nosec G115 // open flags are small non-negative constants; int->uint64 only widens.
		Mode:    uint64(FileMode),                                               //#nosec G115 // the file mode is a small non-negative constant; int->uint64 only widens.
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, classifyOpenErr(err)
	}
	f := os.NewFile(uintptr(fd), rel) //#nosec G115 // a file descriptor is a small non-negative int; int->uintptr only widens.

	if err := stampNode(s.stamps, f, fileStamp()); err != nil {
		// The cleanup unlinks the node the create left behind through the
		// pinned parent, so even it never resolves through a link. Its own
		// failure — the filesystem refusing the removal of a node it just let
		// us create — is a condition CI cannot produce; the join keeps it with
		// the decisive stamp error either way.
		unlinkStamped := func() error {
			pfd, leaf, err := s.openParent(rel)
			if err != nil {
				return err
			}
			defer func() { _ = unix.Close(pfd) }()
			return unix.Unlinkat(pfd, leaf, 0)
		}
		if isXattrUnsupported(err) {
			return nil, errors.Join(fmt.Errorf("could not stamp %s: %w", rel, ErrNoStampSupport), f.Close(), unlinkStamped())
		}
		return nil, errors.Join(err, f.Close(), unlinkStamped())
	}
	return f, nil
}

// openParent resolves every component of rel but the leaf with the same
// categorical refusal the open uses, answering a directory descriptor pinned
// on the verified parent. The name-based operations — creation and rename —
// have no resolve flags of their own, so their containment lives on the
// parent they are issued against; the leaf they act on answers EEXIST or its
// own errno, never a resolution through a link. The caller holds mu: the
// resolution must drain before release closes the descriptor it starts from.
func (s *doubleSys) openParent(rel string) (fd int, leaf string, err error) {
	fd, err = unix.Openat2(s.rootFD, filepath.Dir(rel), &unix.OpenHow{
		Flags:   uint64(os.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC), //#nosec G115 // open flags are small non-negative constants; int->uint64 only widens.
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return 0, "", classifyOpenErr(err)
	}
	return fd, filepath.Base(rel), nil
}

// stampDir is a no-op: directories carry no stamp on the double. The
// stamp's purpose is the projection — mapping a first-level sub-tree root to
// root inside an instance — and nothing off Windows projects a directory, so
// a stamp would be inert. The consequence is adopted deliberately: a
// read-only adopted root still opens, where the Windows stamp needs write
// access to the node and answers a refusal.
func (s *doubleSys) stampDir(string) error {
	return nil
}

// openNode opens the node standing at rel with the profile's access, never the
// target of a link: O_NOFOLLOW answers ELOOP, and a link escaping the root
// is answered by the containment error before the flag is consulted.
// Both mean a link, and both answer ErrPathEscapes.
//
// os.Root cannot be asked not to follow a link resolving inside the tree — it
// resolves one itself — so the double performs its own openat against the
// root's descriptor, the same way os.Root would, with O_NOFOLLOW added.
func (s *doubleSys) openNode(rel string, profile openProfile) (*os.File, error) {
	// Held through the Openat2 below: the descriptor it resolves against must
	// outlive the syscall, or its number can be reused under us.
	s.mu.Lock()
	defer s.mu.Unlock()

	var flags int
	switch profile {
	case openRead, openRename:
		// Rename also moves directories, and they carry no file stamp: the
		// policy decides about them from the Stat, not the kind.
		flags = os.O_RDONLY
	case openAppend:
		flags = os.O_WRONLY | os.O_APPEND
	case openDir:
		flags = os.O_RDONLY | unix.O_DIRECTORY
	}

	// openat2 builds the refusal into the resolution: RESOLVE_NO_SYMLINKS
	// refuses a link at any component of the name, intermediate or leaf — the
	// categorical verdict the Windows open gives through OBJ_DONT_REPARSE,
	// where openat's O_NOFOLLOW guarded the final component alone.
	// RESOLVE_BENEATH backstops the lexical checks: an absolute component or
	// an escaping ".." answers EXDEV instead of resolving out of the tree.
	// No fallback on a kernel without openat2 — the open fails loudly rather
	// than following anything.
	fd, err := unix.Openat2(s.rootFD, rel, &unix.OpenHow{
		Flags:   uint64(flags | unix.O_NOFOLLOW | unix.O_CLOEXEC), //#nosec G115 // flags are the small non-negative O_* constants assembled above; the conversion cannot overflow.
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, classifyOpenErr(err)
	}
	return os.NewFile(uintptr(fd), rel), nil //#nosec G115 // a file descriptor is a small non-negative int; int->uintptr only widens.
}

// classifyOpenErr turns an open refusal into the custodian's answers: a link
// at any component of the name (ELOOP), a missing directory where one was
// needed (ENOTDIR), and RESOLVE_BENEATH's backstop for what the lexical
// checks should never let reach the kernel (EXDEV) are the escape sentinel;
// anything else is the filesystem error it is.
func classifyOpenErr(err error) error {
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.EXDEV) {
		return ErrPathEscapes
	}
	return err
}

// readStamp reads the ownership stamp of an open node. A missing attribute is
// the verdict "this node carries none", not a failure to read one; any other
// failure cannot answer and is returned as the error.
func (s *doubleSys) readStamp(f *os.File) (st stamp, ok bool, err error) {
	// Oversized against the stamp so a tamperer's value of any shape is read
	// whole and judged, not answered by a buffer-size error.
	buf := make([]byte, 32)
	n, err := s.stamps.get(int(f.Fd()), buf) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
	if err != nil {
		if errors.Is(err, unix.ENODATA) {
			return stamp{}, false, nil
		}
		if isXattrUnsupported(err) {
			// Settled when the root was established, so reaching this means
			// the node moved under us. Wrapped, not reworded, so a caller can
			// recognise it.
			return stamp{}, false, fmt.Errorf("%w: %v", ErrNoStampSupport, err)
		}
		return stamp{}, false, err
	}
	if n != 12 {
		return stamp{}, false, nil
	}
	return stamp{
		uid:  binary.LittleEndian.Uint32(buf[0:4]),
		gid:  binary.LittleEndian.Uint32(buf[4:8]),
		mode: binary.LittleEndian.Uint32(buf[8:12]),
	}, true, nil
}

// renameReplace renames by name, as POSIX does: an open destination never
// refuses, and there is no degraded class to fall back from, so the posix
// flag — Windows' concern — answers the same syscall either way. The caller
// guarantees the name still holds the node it verified: nothing unprivileged
// can write inside a stamped tree (ADR 2.01), and the runtime root-swap
// window is closed by instance-side validation.
func (s *doubleSys) renameReplace(f *os.File, oldRel, newRel string, posix bool) error {
	_ = f     // POSIX renames by name, never through the descriptor
	_ = posix // POSIX semantics are native; there is no degraded class
	// Held through the openat2 calls below for the same reason as openNode's.
	s.mu.Lock()
	defer s.mu.Unlock()
	// renameat between pinned parents: the move is issued against
	// resolutions that refused every link, the way the Windows rename is
	// through OBJ_DONT_REPARSE, instead of re-resolving both names from the
	// root the way os.Root.Rename does — following an in-tree link on either
	// side. The leaf itself is a directory entry to move, never a target to
	// resolve: a link standing at the source name moves as a link, and the
	// destination leaf is replaced, both exactly as the policy expects.
	srcFD, srcLeaf, err := s.openParent(oldRel)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(srcFD) }()
	dstFD, dstLeaf, err := s.openParent(newRel)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(dstFD) }()
	return classifyOpenErr(unix.Renameat(srcFD, srcLeaf, dstFD, dstLeaf))
}

// isRenameClassUnsupported answers false everywhere but Windows: POSIX rename
// has no information classes to lack.
func (s *doubleSys) isRenameClassUnsupported(err error) bool {
	return false
}

// isHeldNode answers false: POSIX cannot refuse an unlink because a descriptor
// is open, so the double never sees the refusal the sentinel names.
func (s *doubleSys) isHeldNode(err error) bool {
	_ = err
	return false
}

// isLink answers whether a symbolic link — the only reparse kind a non-
// Windows filesystem carries — stands at rel, classified by the node's own
// mode, never by resolving it: an open follows a relative in-tree link to
// its target and answers ENOENT for a dangling one, so neither form would
// classify the link. The walk to the leaf is the open's own pinned-parent
// resolution, refusing a link at any earlier component the same way, so the
// question is answered for the whole name or not at all — the not-at-all
// case answers the walk's own refusal, which the caller treats as a
// question it could not ask.
func (s *doubleSys) isLink(rel string) (bool, error) {
	// Held through the openat2 below for the same reason as openNode's.
	s.mu.Lock()
	defer s.mu.Unlock()

	pfd, leaf, err := s.openParent(rel)
	if err != nil {
		return false, err
	}
	defer func() { _ = unix.Close(pfd) }()

	var st unix.Stat_t
	if err := unix.Fstatat(pfd, leaf, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return false, classifyOpenErr(err)
	}
	return st.Mode&unix.S_IFMT == unix.S_IFLNK, nil
}

// finalPath answers the path the double was constructed at: POSIX keeps no
// second, final name for a directory, so that is as resolved as this platform
// gets.
func (s *doubleSys) finalPath() string {
	return s.basePath
}

// remoteVolume answers the UNC verdict lexically, exactly as the Windows one
// does: the verdict needs no operating system, and the custodian's contract
// is platform-deterministic. Everything else reports a local volume: the
// custodian's remaining projection concerns are Windows-specific.
func (s *doubleSys) remoteVolume(path string) (bool, volumeKind) {
	return lexicalRemoteVolume(path)
}

// stampNode writes the stamp on the open node: the owner, group and mode, each
// a little-endian uint32, the same values the Windows layer writes as NT
// extended attributes.
func stampNode(stamps stampCalls, f *os.File, st stamp) error {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[0:4], st.uid)
	binary.LittleEndian.PutUint32(b[4:8], st.gid)
	binary.LittleEndian.PutUint32(b[8:12], st.mode)
	return stamps.set(int(f.Fd()), b[:]) //#nosec G115 // a file descriptor is a small non-negative int; uintptr->int is the os.File.Fd contract.
}

// isXattrUnsupported reports whether err says the filesystem cannot carry
// extended attributes at all, as opposed to refusing this particular one.
func isXattrUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
