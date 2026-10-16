package securefiles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// windowsSys is the platformSys of the shipped agent: the NT mechanisms every
// custodian decision in custodian.go executes by. One syscall per method, or
// a compound whose interleaving is the security property itself.
type windowsSys struct {
	mu sync.Mutex
	nt ntCalls

	// rootHandle is the tree root held for the custodian's lifetime without
	// sharing deletion, so nothing else can rename or remove the tree while
	// the custodian serves: a rename and a removal alike must first open the
	// root with DELETE access, and the kernel refuses such an open while any
	// held handle declines to share deletion. The veto is share arbitration
	// rather than an ACL, so it binds every issuer the same way — a Windows
	// caller or the 9p server that performs an instance's mv — and no right the
	// same user holds over the parent overrides it: measured against a held
	// root, a WSL-issued mv and rm -rf are refused. POSIX cannot refuse a
	// rename because a descriptor is open, so the other platforms hold
	// nothing, and the runtime root-swap window there is closed by the
	// instance-side validation instead (ADR 2.01). It is also the base every
	// relative NT open resolves against. Invalid until the hold is taken.
	rootHandle windows.Handle

	// basePath is the path the layer was constructed at, recorded so
	// establishRoot can establish the root under its parent and finalPath can
	// answer the name back.
	basePath string
}

// fileRenameInfoStruct is FILE_RENAME_INFORMATION. Its leading word is a BOOLEAN
// ReplaceIfExists for FileRenameInformation and a ULONG of flags for
// FileRenameInformationEx; the documented struct overlaps them in a union, and
// FILE_RENAME_REPLACE_IF_EXISTS is 1, so one field serves both classes.
type fileRenameInfoStruct struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// fileRenameInformationEx is the FILE_INFORMATION_CLASS of
// FILE_RENAME_INFORMATION_EX, which golang.org/x/sys/windows does not declare.
// Unlike FileRenameInformation it honours FILE_RENAME_POSIX_SEMANTICS, without
// which the kernel refuses the rename whenever any process holds the
// destination open, however politely it shares it.
const fileRenameInformationEx = 65

// ntCalls is the NT surface the mechanisms use. It is a field of windowsSys
// rather than a set of package-level variables so that there is no
// process-wide switch an attacker could flip to turn stamping off for every
// custodian at once: it is set once at construction and never reassigned.
// Production wires realNtCalls; the tests in this package wire a fake to
// reproduce the conditions only a filesystem can create — extended attributes
// refused, and a rename information class the volume does not implement.
type ntCalls struct {
	setEaFile          func(h windows.Handle, ea []byte) error
	queryEaFile        func(h windows.Handle, buf []byte) (int, error)
	createFile         func(handle *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, attrs, share, disp, opts uint32, ea []byte) error
	setInformationFile func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error
	// finalPath is the handle's final path: the query CheckProjection
	// classifies the sub-tree by. It is a seam so the tests can reproduce a
	// root the kernel resolves into a share, a condition no CI machine can
	// produce.
	finalPath func(h windows.Handle, buf *uint16, maxLen uint32) (uint32, error)
	// getDriveType classifies a volume root the way GetDriveType does. It is
	// a seam so the tests can pin the mapped-network-drive verdict, a machine
	// state no CI machine has.
	getDriveType func(root *uint16) uint32
}

// fileIDInfo mirrors FILE_ID_INFO, which golang.org/x/sys/windows does not
// declare.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

// realNtCalls returns the production implementation: the NT calls themselves.
func realNtCalls() ntCalls {
	return ntCalls{
		setEaFile: func(h windows.Handle, ea []byte) error {
			var iosb windows.IO_STATUS_BLOCK
			return windows.NtSetEaFile(h, &iosb, &ea[0], uint32(len(ea))) //#nosec G115 // length of a small EA buffer; always fits in 32 bits.
		},
		queryEaFile: func(h windows.Handle, buf []byte) (int, error) {
			var iosb windows.IO_STATUS_BLOCK
			err := windows.NtQueryEaFile(h, &iosb, &buf[0], uint32(len(buf)), false, nil, 0, nil, true) //#nosec G115 // length of a small EA buffer; always fits in 32 bits.
			return int(iosb.Information), err                                                           //#nosec G115 // a successful query fills at most len(buf); int is Go's natural slice length.
		},
		createFile: func(handle *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, attrs, share, disp, opts uint32, ea []byte) error {
			var eaBuf uintptr
			if len(ea) > 0 {
				eaBuf = uintptr(unsafe.Pointer(&ea[0])) //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			}
			return windows.NtCreateFile(
				handle, access, oa, iosb, nil, attrs, share, disp, opts,
				eaBuf,
				uint32(len(ea)), //#nosec G115 // length of a small EA buffer; always fits in 32 bits.
			)
		},
		setInformationFile: func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error {
			return windows.NtSetInformationFile(h, iosb, &buf[0], uint32(len(buf)), class) //#nosec G115 // rename-info buffer; always fits in 32 bits.
		},
		finalPath: func(h windows.Handle, buf *uint16, maxLen uint32) (uint32, error) {
			return windows.GetFinalPathNameByHandle(h, buf, maxLen, 0)
		},
		getDriveType: func(root *uint16) uint32 {
			return windows.GetDriveType(root)
		},
	}
}

// newPlatformSys returns the Windows layer for the path open() was handed.
// Production wires the NT calls themselves; the tests in this package wire a
// fake, through newWindowsSysWith.
func newPlatformSys(basePath string) platformSys {
	return newWindowsSysWith(basePath, realNtCalls())
}

// newWindowsSysWith is newPlatformSys with the NT surface supplied rather than
// assumed.
func newWindowsSysWith(basePath string, nt ntCalls) platformSys {
	return &windowsSys{
		rootHandle: windows.InvalidHandle,
		nt:         nt,
		basePath:   basePath,
	}
}

// sub returns the layer for a sub-custodian: the same NT surface as the
// parent's, as it stands at the handout, and no hold of its own yet — attach
// takes it beside the node open() verifies.
func (s *windowsSys) sub() platformSys {
	return &windowsSys{
		rootHandle: windows.InvalidHandle,
		nt:         s.nt,
		basePath:   s.basePath,
	}
}

// establishRoot brings the tree root into existence — created fresh under its
// parent, or adopted if an earlier run left it — and stamps it before it
// answers. Three properties are deliberate:
//   - the stamp rides the creating syscall, so the root never exists
//     unstamped, not even for an instant;
//   - the same open declines to share deletion and stays open for the
//     custodian's lifetime, which is the veto that keeps the tree from being
//     renamed or removed while the custodian serves (see rootHandle);
//   - a root that cannot be stamped is refused, not served unstamped
//     (ADR 2.02).
//
// It answers the identity it established, for open() to verify against its
// second, independent resolution of the path.
func (s *windowsSys) establishRoot() (id fileIdentity, known bool, err error) {
	parentDir := filepath.Dir(s.basePath)
	baseName := filepath.Base(s.basePath)

	parentPath16, err := windows.UTF16PtrFromString(parentDir)
	if err != nil {
		return fileIdentity{}, false, err
	}

	parentHandle, err := windows.CreateFile(
		parentPath16,
		windows.GENERIC_READ|windows.FILE_WRITE_DATA|windows.FILE_APPEND_DATA|windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fileIdentity{}, false, err
	}
	defer closeHandle(parentHandle)

	eaBuf := encodeLxEa(dirStamp())

	oa, err := relativeAttributes(parentHandle, baseName)
	if err != nil {
		return fileIdentity{}, false, err
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	// The handle must not hold DELETE access itself: sharing is checked in
	// both directions, so a later open that does not share deletion
	// (os.OpenRoot, for one) would be refused against this handle for as
	// long as it lives. The veto lives in the share mode alone.
	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.SYNCHRONIZE | windows.FILE_WRITE_EA)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_DIRECTORY)
	// The same open that stamps the root holds it: without sharing deletion,
	// nothing else can rename or remove the tree from this syscall on. See
	// rootHandle.
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	createOptions := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DIRECTORY_FILE)
	disposition := uint32(windows.FILE_OPEN_IF)

	// A root that cannot be stamped is refused, not adopted. Everything it
	// would hold is a credential the agent manages, and a sub-tree no instance
	// sees as root-owned publishes those to every unprivileged process in
	// every instance. Refusing is also what keeps a redirected root refused:
	// OBJ_DONT_REPARSE is what failed this call rather than follow a link
	// standing where the root should be.
	err = s.nt.createFile(&handle, desiredAccess, oa, &iosb, fileAttributes, shareAccess, disposition, createOptions, eaBuf)
	if err != nil {
		return fileIdentity{}, false, mapNtStatus(err)
	}

	// If opening a pre-existing root directory (iosb.Information == 1 ->
	// FILE_OPENED), NtCreateFile does not apply the eaBuf parameter. Stamp
	// the EA via NtSetEaFile: ADR 2.01 requires the root to carry the stamp
	// even when adopted.
	if iosb.Information == 1 /* FILE_OPENED */ {
		if err := s.nt.setEaFile(handle, eaBuf); err != nil {
			closeHandle(handle)
			return fileIdentity{}, false, fmt.Errorf("could not stamp the pre-existing root: %w", mapNtStatus(err))
		}
	}

	// Record what was created, so the reopen in open() can be tied back to it.
	id, known = identityOf(handle)

	// The stamping open is the hold: it stays open for the custodian's
	// lifetime and doubles as the base every relative open resolves against.
	s.rootHandle = handle
	return id, known, nil
}

// attach takes the hold on the tree root that keeps it from being renamed or
// removed while the custodian serves (see rootHandle). It exists for
// sub-custodians, whose roots never went through establishRoot: a
// path-established root is already held by its stamping open, a sub-tree
// root would serve unheld. The hold is taken beside the node the
// verification just answered for — through its handle, never by resolving a
// path — and attaching twice is a no-op.
func (s *windowsSys) attach(root *os.Root) error {
	if s.rootHandle != windows.InvalidHandle {
		return nil
	}

	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return s.holdNode(windows.Handle(f.Fd()))
}

// release releases the hold on the tree root. Served operations read it and
// Close writes it, so the mutex guards the pair: an in-flight operation
// drains before the handle closes, and every operation after answers the
// invalid handle instead of racing the close.
func (s *windowsSys) release() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rootHandle != windows.InvalidHandle {
		closeHandle(s.rootHandle)
		s.rootHandle = windows.InvalidHandle
	}
	return nil
}

// identity answers the filesystem identity of an open node: FileIdInfo first,
// because its 128-bit ID is the real one — ReFS reports a narrower derived
// value through GetFileInformationByHandle — and a filesystem that supplies
// no identity is not an error; it only means the node cannot be re-identified
// later.
func (s *windowsSys) identity(f *os.File) (fileIdentity, bool) {
	return identityOf(windows.Handle(f.Fd()))
}

// createStamped creates rel under the tree root stamped as the custodian's
// own: the stamp rides on the creating syscall, so a directory (which has no
// descriptor to hand out and returns nil) is stamped the moment it exists,
// and a file's descriptor is the very node the stamp was applied to. No name
// resolution happens between the stamp and the caller's writes, so ownership
// cannot be swapped out from under the descriptor. The caller owns the
// returned file.
func (s *windowsSys) createStamped(rel string, dir bool) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := fileStamp()
	if dir {
		st = dirStamp()
	}

	eaBuf := encodeLxEa(st)

	oa, err := relativeAttributes(s.rootHandle, rel)
	if err != nil {
		return nil, err
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createDisposition := uint32(windows.FILE_CREATE)
	createOptions := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT)

	if dir {
		createOptions |= windows.FILE_DIRECTORY_FILE
		fileAttributes = windows.FILE_ATTRIBUTE_DIRECTORY
	} else {
		createOptions |= windows.FILE_NON_DIRECTORY_FILE
	}

	ntErr := s.nt.createFile(&handle, desiredAccess, oa, &iosb, fileAttributes, shareAccess, createDisposition, createOptions, eaBuf)
	if ntErr != nil {
		return nil, mapNtStatus(ntErr)
	}

	if dir {
		closeHandle(handle)
		return nil, nil
	}
	return os.NewFile(uintptr(handle), rel), nil
}

// stampDir stamps an adopted directory in place: ADR 2.01 requires first-level
// sub-tree roots to carry the stamp even when pre-existing, because it is
// what revokes unprivileged creation and deletion inside them. The node itself
// is opened, never the target of a link: adoption stamps what stands at the
// name.
func (s *windowsSys) stampDir(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	eaBuf := encodeLxEa(dirStamp())

	handle, err := s.openExisting(rel, windows.GENERIC_WRITE|windows.FILE_WRITE_EA,
		windows.FILE_ATTRIBUTE_DIRECTORY, windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT)
	if err != nil {
		return err
	}
	defer closeHandle(handle)

	if err := s.nt.setEaFile(handle, eaBuf); err != nil {
		return fmt.Errorf("could not stamp the sub-directory %s: %w", rel, mapNtStatus(err))
	}
	return nil
}

// openNode opens the node standing at rel with the profile's access, never the
// target of a reparse point: OBJ_DONT_REPARSE answers a link standing there
// with ErrPathEscapes. openRename adds the access the publish path needs, so
// that the verification and the rename answer for the same node. The append
// profile asks for FILE_APPEND_DATA without FILE_WRITE_DATA, so the kernel
// itself routes every write on the descriptor to the node's end — a handle
// cannot seek-and-overwrite, whatever the holder tries. The directory profile
// asks for listing rights on a directory and nothing else.
func (s *windowsSys) openNode(rel string, profile openProfile) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		access     uint32
		attributes uint32
		options    uint32
	)
	switch profile {
	case openRead:
		access = windows.GENERIC_READ
	case openRename:
		access = windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE
	case openAppend:
		// No FILE_WRITE_DATA: append rights alone make the kernel perform
		// every write at the end of the file, the access-right equivalent of
		// POSIX O_APPEND. And FILE_APPEND_DATA on a directory is the right to
		// create a subdirectory, not a refusal, so a non-directory node must
		// be demanded explicitly: a directory standing at the name — a real
		// one, or a link's target — answers STATUS_FILE_IS_A_DIRECTORY.
		access = windows.GENERIC_READ | windows.FILE_APPEND_DATA
		options = windows.FILE_NON_DIRECTORY_FILE
	case openDir:
		access = windows.FILE_LIST_DIRECTORY
		// No FILE_ATTRIBUTE_DIRECTORY: attributes are for creation, and an
		// open answers ERROR_INVALID_NAME for them. FILE_DIRECTORY_FILE in
		// the options is what asks for a directory.
		options = windows.FILE_DIRECTORY_FILE
	}

	// The NT object manager has no "." name: an empty relative name opens
	// the base the OBJECT_ATTRIBUTES anchors on (the same rule holdNode
	// uses), so the tree root itself is named the way it resolves.
	if rel == "." {
		rel = ""
	}

	h, err := s.openExisting(rel, access, attributes, options)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), rel), nil
}

// readStamp reads the $LXUID, $LXGID and $LXMOD extended attributes of the
// node behind f. Windows has no empty-list answer: a node without attributes
// fails the query itself, and that failure is the no-stamp verdict about the
// node, not a failure of it; an incomplete stamp decodes fine and answers the
// same. Any other status cannot answer and is returned as the error.
func (s *windowsSys) readStamp(f *os.File) (st stamp, ok bool, err error) {
	buf := make([]byte, 2048)
	n, err := s.nt.queryEaFile(windows.Handle(f.Fd()), buf)
	if err != nil {
		if status, ok := errors.AsType[windows.NTStatus](err); ok && status == windows.STATUS_NO_EAS_ON_FILE {
			return stamp{}, false, nil
		}
		return stamp{}, false, mapNtStatus(err)
	}

	eas, err := winio.DecodeExtendedAttributes(buf[:n])
	if err != nil {
		return stamp{}, false, err
	}

	var found stamp
	var haveUID, haveGID, haveMode bool
	for _, ea := range eas {
		if len(ea.Value) == 4 {
			val := binary.LittleEndian.Uint32(ea.Value)
			switch ea.Name {
			case "$LXUID":
				found.uid, haveUID = val, true
			case "$LXGID":
				found.gid, haveGID = val, true
			case "$LXMOD":
				found.mode, haveMode = val, true
			}
		}
	}
	if !haveUID || !haveGID || !haveMode {
		return stamp{}, false, nil
	}
	return found, true, nil
}

// renameReplace renames the open node to newRel, replacing whatever stands
// there, through the FILE_RENAME_INFORMATION class the caller selected.
// POSIX semantics is what allows the replacement to happen while readers still
// hold the destination open, which is the whole point of publishing by
// rename. The destination's parent is resolved through the tree root, without
// following reparse points.
func (s *windowsSys) renameReplace(f *os.File, oldRel, newRel string, posix bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	handle := windows.Handle(f.Fd())

	// Resolve the destination parent directory handle safely without
	// following reparse points.
	dirPart := filepath.Dir(newRel)
	leafPart := filepath.Base(newRel)

	targetParentHandle := s.rootHandle
	if dirPart != "." && dirPart != "" {
		parentH, err := s.openExisting(dirPart, windows.GENERIC_READ|windows.FILE_LIST_DIRECTORY,
			windows.FILE_ATTRIBUTE_DIRECTORY, windows.FILE_DIRECTORY_FILE)
		if err != nil {
			return err
		}
		defer closeHandle(parentH)
		targetParentHandle = parentH
	}

	leaf16, err := windows.UTF16FromString(leafPart)
	if err != nil {
		return err
	}
	name := leaf16[:len(leaf16)-1] // drop the NUL terminator

	nameBytesLen := len(name) * 2
	infoSize := unsafe.Sizeof(fileRenameInfoStruct{}) + uintptr(nameBytesLen) - 2

	buf := make([]byte, infoSize)
	info := (*fileRenameInfoStruct)(unsafe.Pointer(&buf[0])) //#nosec G103 // reinterpreting the rename-info buffer as its documented header; the buffer is sized to hold it.
	info.RootDirectory = targetParentHandle
	info.FileNameLength = uint32(nameBytesLen)             //#nosec G115 // rename target byte length; a short relative path, always fits in 32 bits.
	copy(unsafe.Slice(&info.FileName[0], len(name)), name) //#nosec G103 // FileName declares only its first element; the slice overlays the buffer's tail, which was sized to hold the name.

	var iosbSet windows.IO_STATUS_BLOCK
	if posix {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
		err := s.nt.setInformationFile(handle, &iosbSet, buf, fileRenameInformationEx)
		if isUnsupportedInfoClass(err) {
			// Unmapped, on purpose: the caller classifies the failure to
			// decide the fallback, and the classification reads the NT status
			// itself — the Errno translation would erase it.
			return err
		}
		return mapNtStatus(err)
	}
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS
	return mapNtStatus(s.nt.setInformationFile(handle, &iosbSet, buf, windows.FileRenameInformation))
}

// isRenameClassUnsupported answers whether err says the filesystem does not
// implement the POSIX rename class at all, as opposed to refusing this
// particular rename. Only then is retrying with the older class worthwhile.
func (s *windowsSys) isRenameClassUnsupported(err error) bool {
	return isUnsupportedInfoClass(err)
}

// isHeldNode answers whether err says the removal was refused because another
// process holds the node open without sharing deletion: the DELETE-access open
// a removal needs was refused by share arbitration. Measured holders that do
// not share deletion are host-side processes opening with plain read sharing,
// such as Go's os.Open; the WSL 9p server opens with full sharing (it must, to
// implement POSIX unlink-while-open), so an instance holding the node never
// lands here.
func (s *windowsSys) isHeldNode(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// isUnsupportedInfoClass reports whether err says the filesystem does not
// implement the information class at all, as opposed to refusing this
// particular operation. Only then is retrying with an older class
// worthwhile.
func isUnsupportedInfoClass(err error) bool {
	return errors.Is(err, windows.STATUS_INVALID_PARAMETER) ||
		errors.Is(err, windows.STATUS_NOT_SUPPORTED) ||
		errors.Is(err, windows.STATUS_INVALID_INFO_CLASS) ||
		errors.Is(err, windows.STATUS_NOT_IMPLEMENTED)
}

// isLink reports whether the node standing at rel is a reparse point of any
// kind. The node itself is opened, never its target, so every link kind is
// classified uniformly: the create call ahead of it answers a name collision
// for some and an escape refusal for others.
func (s *windowsSys) isLink(rel string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handle, err := s.openExisting(rel, windows.FILE_READ_ATTRIBUTES, 0, windows.FILE_OPEN_REPARSE_POINT)
	if err != nil {
		return false, err
	}
	defer closeHandle(handle)

	var info struct {
		FileAttributes uint32
		ReparseTag     uint32
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo,
		(*byte)(unsafe.Pointer(&info)), //#nosec G103 // the API writes the documented struct into this buffer.
		uint32(unsafe.Sizeof(info))); err != nil {
		return false, err
	}
	return info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

// finalPath answers the tree root as the filesystem finally names it, through
// the held root handle. MAX_LONG_PATH covers every path the kernel forms, so
// the retry below is a safety net for volumes that over-answer, not the
// expected route: falling back to "" would silently hide a remote base from
// CheckProjection.
func (s *windowsSys) finalPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rootHandle == windows.InvalidHandle {
		return ""
	}

	buf := make([]uint16, windows.MAX_LONG_PATH)
	for {
		n, err := s.nt.finalPath(s.rootHandle, &buf[0], uint32(len(buf)))              //#nosec G115 // buffer length; always fits in 32 bits.
		if errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) && n > uint32(len(buf)) { //#nosec G115 // the failure names the size needed; always fits.
			buf = make([]uint16, n)
			continue
		}
		if err != nil || n == 0 {
			return ""
		}
		return windows.UTF16ToString(buf[:n])
	}
}

// holdNode takes the rootHandle hold beside the node an already-open handle
// stands for, for custodians whose root was never resolved by a path: a
// sub-custodian reaches its root through the parent, so its hold is taken
// relative to the verified node instead of at a stamping open. The veto
// semantics are the rootHandle field's. An empty relative name opens the base
// itself: NtCreateFile resolves it against the RootDirectory of the
// OBJECT_ATTRIBUTES, which is the verified node.
func (s *windowsSys) holdNode(base windows.Handle) error {
	oa, err := relativeAttributes(base, "")
	if err != nil {
		return err
	}

	var iosb windows.IO_STATUS_BLOCK
	var h windows.Handle
	err = s.nt.createFile(
		&h,
		windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		oa,
		&iosb,
		windows.FILE_ATTRIBUTE_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		nil,
	)
	if err != nil {
		return mapNtStatus(err)
	}

	s.rootHandle = h
	return nil
}

// openExisting opens an existing node relative to the custodian's root handle.
// Every NT open in this file wants the same things — no reparse point may be
// followed, the node is shared with other readers and writers, and the handle
// is synchronous — so they are settled here rather than repeated at each call
// site, where one omission would be a silent hole.
func (s *windowsSys) openExisting(rel string, access, attributes, options uint32) (windows.Handle, error) {
	oa, err := relativeAttributes(s.rootHandle, rel)
	if err != nil {
		return windows.InvalidHandle, err
	}

	var iosb windows.IO_STATUS_BLOCK
	var h windows.Handle

	if err := windows.NtCreateFile(
		&h,
		access|windows.SYNCHRONIZE,
		oa,
		&iosb,
		nil,
		attributes,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		options|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	); err != nil {
		return windows.InvalidHandle, mapNtStatus(err)
	}

	return h, nil
}

// mapNtStatus translates an NT status into a Go error, recognising the two
// conditions the custodian names for itself: a rooted, OBJ_DONT_REPARSE
// syscall blocked by a symlink crossing the custodian root, and a filesystem
// that cannot carry the stamp at all. The statuses for the second were
// measured against a volume that cannot store extended attributes, not
// assumed: creating a node with an attribute buffer answers
// STATUS_EAS_NOT_SUPPORTED, while setting one on a node already there answers
// STATUS_INVALID_DEVICE_REQUEST. STATUS_INVALID_PARAMETER is excluded on
// purpose: no such volume returns it, but our own buffer or flags would if
// they ever became wrong, and reading that as a property of the filesystem
// would blame the machine for our mistake. A malformed buffer answers
// STATUS_EA_LIST_INCONSISTENT.
func mapNtStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.STATUS_STOPPED_ON_SYMLINK) || errors.Is(err, windows.STATUS_REPARSE_POINT_ENCOUNTERED) {
		return ErrPathEscapes
	}
	if errors.Is(err, windows.STATUS_EAS_NOT_SUPPORTED) ||
		errors.Is(err, windows.STATUS_INVALID_DEVICE_REQUEST) ||
		errors.Is(err, windows.STATUS_NOT_SUPPORTED) {
		// The status underneath is for the human reading the refusal; no
		// caller matches it, only the sentinel.
		return fmt.Errorf("%w: %v", ErrNoStampSupport, err)
	}
	if ntstatus, ok := errors.AsType[windows.NTStatus](err); ok {
		return ntstatus.Errno()
	}
	return err
}

// identityOf reports the identity of the node behind h, and whether the
// filesystem supplied a usable one. FileIdInfo is asked first because its
// 128-bit ID is the real one: ReFS reports a narrower derived value through
// GetFileInformationByHandle, so comparing the 64-bit form there is weaker
// than it looks.
func identityOf(h windows.Handle) (fileIdentity, bool) {
	var info fileIDInfo
	if err := windows.GetFileInformationByHandleEx(
		h,
		windows.FileIdInfo,
		(*byte)(unsafe.Pointer(&info)), //#nosec G103 // the API writes the documented struct into this buffer.
		uint32(unsafe.Sizeof(info)),
	); err == nil {
		id := fileIdentity{volume: info.VolumeSerialNumber, file: info.FileID}
		return id, id.file != [16]byte{}
	}

	var byHandle windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &byHandle); err != nil {
		return fileIdentity{}, false
	}

	id := fileIdentity{volume: uint64(byHandle.VolumeSerialNumber)}
	binary.LittleEndian.PutUint64(id.file[:8], uint64(byHandle.FileIndexHigh)<<32|uint64(byHandle.FileIndexLow))

	return id, id.file != [16]byte{}
}

// closeHandle closes a Windows handle and discards its error, used in cleanup
// paths where propagation is not useful.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
}

// remoteVolume reports whether path lives on a volume this machine does not
// own, and names the kind. A UNC path is classified by its prefix, not
// GetDriveType: for an unreachable server that call blocks on name resolution
// for seconds and then answers DRIVE_NO_ROOT_DIR anyway. Drive letters are
// cheap, so they go through the API, which is what identifies a mapped network
// drive. The path must already be resolved: a directory in the profile can be
// a link to a share, where the name says "C:" while the bytes live on a server.
func (s *windowsSys) remoteVolume(path string) (remote bool, kind volumeKind) {
	remote, kind = lexicalRemoteVolume(path)
	if remote {
		return remote, kind
	}
	vol := filepath.VolumeName(stripNTPrefix(path))
	if vol == "" {
		return false, 0
	}

	root, err := windows.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return false, 0
	}
	if s.nt.getDriveType(root) == windows.DRIVE_REMOTE {
		return true, volumeMappedDrive
	}

	return false, 0
}

// dirStamp is the stamp an adopted sub-tree root carries: a directory,
// owned by root, mode 0700. Only the Windows layer stamps directories —
// the projection that makes the stamp meaningful is Windows-side too —
// so this is not part of the cross-platform sys.go.
func dirStamp() stamp {
	return stamp{uid: 0, gid: 0, mode: 040700}
}

// encodeLxEa encodes the stamp as the three WSL projection attributes the
// instances read. It cannot fail: the names are fixed constants an EA name
// always carries, and the values are 4 bytes each, far under any EA size
// limit, so the winio encoder's size rejections are unreachable by
// construction. Should a future change make that untrue — a longer name, a
// variable-size value — the panic fires in tests, and reintroducing an error
// return is the compiler-guided decision it deserves to be.
func encodeLxEa(st stamp) []byte {
	var uidBytes, gidBytes, modeBytes [4]byte
	binary.LittleEndian.PutUint32(uidBytes[:], st.uid)
	binary.LittleEndian.PutUint32(gidBytes[:], st.gid)
	binary.LittleEndian.PutUint32(modeBytes[:], st.mode)

	eas := []winio.ExtendedAttribute{
		{Name: "$LXUID", Value: uidBytes[:]},
		{Name: "$LXGID", Value: gidBytes[:]},
		{Name: "$LXMOD", Value: modeBytes[:]},
	}
	buf, err := winio.EncodeExtendedAttributes(eas)
	if err != nil {
		panic("encodeLxEa: " + err.Error()) // unreachable: fixed names, 4-byte values
	}
	return buf
}

// relativeAttributes builds the OBJECT_ATTRIBUTES that open the relative
// name rel against the root handle: the kernel resolves rel inside the tree
// the handle stands for, never from the drive root or the current directory.
// Every NT open in this file goes through it, and OBJ_DONT_REPARSE is set
// here, so none can follow a reparse point planted inside the sub-tree.
func relativeAttributes(root windows.Handle, rel string) (*windows.OBJECT_ATTRIBUTES, error) {
	name, err := windows.NewNTUnicodeString(rel)
	if err != nil {
		return nil, err
	}

	return &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: root,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}, nil
}
