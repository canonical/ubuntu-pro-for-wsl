//go:build windows

package securefiles

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

type platformSys struct {
	mu         sync.Mutex
	root       *os.Root
	rootFile   *os.File
	rootHandle windows.Handle
	degraded   bool
	mockOwned  *bool
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

// fileRenameInformationEx is the FILE_INFORMATION_CLASS of FILE_RENAME_INFORMATION_EX,
// which golang.org/x/sys/windows does not declare. Unlike FileRenameInformation it
// honours FILE_RENAME_POSIX_SEMANTICS, without which the kernel refuses the rename
// whenever any process holds the destination open, however politely it shares it.
const fileRenameInformationEx = 65

// testNtSetEaFileResult, when non-nil, overrides the return value of NtSetEaFile in tests.
var testNtSetEaFileResult *uint32

// testNtCreateFileResult, when non-nil, overrides the return value of NtCreateFile
// in createNode during tests, so creation failure paths can be exercised without
// sabotaging the root the custodian was opened on.
var testNtCreateFileResult *uint32

func newPlatformSys(basePath string) (*platformSys, error) {
	s := &platformSys{
		rootHandle: windows.InvalidHandle,
	}

	if err := s.ensureRoot(basePath); err != nil {
		return nil, err
	}

	return s, nil
}

// newSubPlatformSys returns a platformSys for a sub-directory the parent custodian has
// already created and stamped. It deliberately does no path walk of its own: the node is
// reached through the parent's root handle, and re-resolving its absolute path here would
// step outside the containment the parent established. Degradation is inherited because
// it describes the filesystem, not the node.
func newSubPlatformSys(degraded bool) *platformSys {
	return &platformSys{
		rootHandle: windows.InvalidHandle,
		degraded:   degraded,
	}
}

// stampSubdir stamps an already-existing sub-directory in place, for the case where the
// custodian adopts one left by an earlier run. ADR 2.01 requires first-level sub-tree
// roots to carry the stamp: on a stamped directory Linux checks the current ownership and
// mode on every operation, so the stamp revokes unprivileged creation and deletion inside
// it. The node is opened relative to the root handle and without following reparse points,
// so adoption cannot be redirected outside the sub-tree. A filesystem that refuses the
// attribute degrades the custodian rather than failing the call, per ADR 2.02.
func (s *platformSys) stampSubdir(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.degraded {
		return nil
	}

	eaBuf, err := encodeLxEa(0, 0, 040700)
	if err != nil {
		return err
	}

	handle, err := s.openExisting(rel, windows.GENERIC_WRITE|windows.FILE_WRITE_EA,
		windows.FILE_ATTRIBUTE_DIRECTORY, windows.FILE_DIRECTORY_FILE)
	if err != nil {
		return err
	}
	defer closeHandle(handle)

	var iosbSet windows.IO_STATUS_BLOCK
	errSet := windows.NtSetEaFile(
		handle,
		&iosbSet,
		&eaBuf[0],
		uint32(len(eaBuf)), //#nosec G115 // length of small EA buffer; always fits in 32 bits.
	)
	if errSet != nil {
		s.degraded = true
	}

	return nil
}

// setRoot derives the root directory handle used for relative NtCreateFile
// calls from the os.Root, so EA-stamped creation is rooted at the same
// directory that provides structural containment.
func (s *platformSys) setRoot(root *os.Root) error {
	s.root = root
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	s.rootFile = f
	s.rootHandle = windows.Handle(f.Fd())
	return nil
}

func (s *platformSys) ensureRoot(basePath string) error {
	parentDir := filepath.Dir(basePath)
	baseName := filepath.Base(basePath)

	if err := os.MkdirAll(parentDir, DirMode); err != nil {
		return err
	}

	parentPath16, err := windows.UTF16PtrFromString(parentDir)
	if err != nil {
		return err
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
		return err
	}
	defer closeHandle(parentHandle)

	eaBuf, err := encodeLxEa(0, 0, 040700)
	if err != nil {
		return err
	}

	oa, err := relativeAttributes(parentHandle, baseName)
	if err != nil {
		return err
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE | windows.FILE_WRITE_EA)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_DIRECTORY)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createOptions := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DIRECTORY_FILE)
	disposition := uint32(windows.FILE_OPEN_IF)

	err = windows.NtCreateFile(
		&handle,
		desiredAccess,
		oa,
		&iosb,
		nil,
		fileAttributes,
		shareAccess,
		disposition,
		createOptions,
		uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uint32(len(eaBuf)),                 //#nosec G115 // length of small EA buffer; always fits in 32 bits.
	)
	if err != nil {
		// OBJ_DONT_REPARSE is what made this call fail rather than quietly follow a
		// junction or symlink standing where the root should be. The fallback below would
		// undo that in one line: os.MkdirAll succeeds on an existing directory link, and
		// this path records no identity, so setRoot would have nothing to compare and the
		// custodian would serve every later operation from outside basePath without a
		// word. A redirected root is a refusal, not a degradation.
		if escapes := mapNtStatus(err); errors.Is(escapes, ErrPathEscapes) {
			return escapes
		}

		// What remains is a filesystem that will not carry the stamp at creation time.
		// That one degrades loudly and keeps serving, per ADR 2.02.
		s.degraded = true
		return os.MkdirAll(basePath, DirMode)
	}

	// If opening a pre-existing root directory (iosb.Information == 1 -> FILE_OPENED),
	// NtCreateFile does not apply the eaBuf parameter. Stamp EA via NtSetEaFile.
	if iosb.Information == 1 /* FILE_OPENED */ {
		var iosbSet windows.IO_STATUS_BLOCK
		var errSet error
		if testNtSetEaFileResult != nil {
			errSet = windows.NTStatus(*testNtSetEaFileResult)
		} else {
			errSet = windows.NtSetEaFile(
				handle,
				&iosbSet,
				&eaBuf[0],
				uint32(len(eaBuf)), //#nosec G115 // length of small EA buffer; always fits in 32 bits.
			)
		}
		if errSet != nil {
			s.degraded = true
		}
	}

	closeHandle(handle)
	return nil
}

func (s *platformSys) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rootFile != nil {
		err := s.rootFile.Close()
		s.rootFile = nil
		s.rootHandle = windows.InvalidHandle
		return err
	}
	return nil
}

func (s *platformSys) isDegraded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.degraded
}

func (s *platformSys) setMockDegraded(degraded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.degraded = degraded
}

// isOwned reports whether the node at rel carries the agent's watermark: the
// $LXUID/$LXGID/$LXMOD stamp queried through NtQueryEaFile for exactly the
// values the custodian writes. It never answers on behalf of a filesystem that
// cannot carry extended attributes: there the query simply fails, and ownership
// is unknowable rather than true. Callers must consult isDegraded first and
// decide what an unverifiable sub-tree means for them, because reading an
// unknowable answer as either "ours" or "foreign" is a policy choice, not a
// fact this predicate can supply.
func (s *platformSys) isOwned(rel string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mockOwned != nil {
		return *s.mockOwned, nil
	}

	h, err := s.openExisting(rel, windows.GENERIC_READ, 0, windows.FILE_NON_DIRECTORY_FILE)
	if err != nil {
		return false, err
	}
	defer closeHandle(h)

	uid, gid, mode, err := ntQueryLxEa(h)
	if err != nil {
		return false, err
	}
	return uid == 0 && gid == 0 && mode == stampedFileMode(), nil
}

func (s *platformSys) setMockOwned(owned *bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mockOwned = owned
}

func (s *platformSys) createNode(relativePath string, isDir bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := stampedFileMode()
	if isDir {
		mode = uint32(040700)
	}

	if s.degraded {
		return fallbackCreate(s.root, relativePath, isDir)
	}

	eaBuf, err := encodeLxEa(0, 0, mode)
	if err != nil {
		return err
	}

	oa, err := relativeAttributes(s.rootHandle, relativePath)
	if err != nil {
		return err
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createDisposition := uint32(windows.FILE_CREATE)
	createOptions := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT)

	if isDir {
		createOptions |= windows.FILE_DIRECTORY_FILE
		fileAttributes = windows.FILE_ATTRIBUTE_DIRECTORY
	} else {
		createOptions |= windows.FILE_NON_DIRECTORY_FILE
	}

	var ntErr error
	if testNtCreateFileResult != nil {
		ntErr = windows.NTStatus(*testNtCreateFileResult)
	} else {
		ntErr = windows.NtCreateFile(
			&handle,
			desiredAccess,
			oa,
			&iosb,
			nil,
			fileAttributes,
			shareAccess,
			createDisposition,
			createOptions,
			uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			uint32(len(eaBuf)),                 //#nosec G115 // length of small EA buffer; always fits in 32 bits.
		)
	}

	if ntErr != nil {
		// A filesystem that will not take the attribute buffer degrades and falls back;
		// every other failure, a redirected path above all, is reported as it is.
		if errors.Is(ntErr, windows.STATUS_EAS_NOT_SUPPORTED) || errors.Is(ntErr, windows.STATUS_NOT_SUPPORTED) || errors.Is(ntErr, windows.STATUS_INVALID_PARAMETER) {
			s.degraded = true
			return fallbackCreate(s.root, relativePath, isDir)
		}
		return mapNtStatus(ntErr)
	}

	closeHandle(handle)
	return nil
}

// openExisting opens an existing node relative to the custodian's root handle. Every NT open
// in this file wants the same things — no reparse point may be followed, the node is shared
// with other readers and writers, and the handle is synchronous — so they are settled here
// rather than repeated at each call site, where one omission would be a silent hole.
func (s *platformSys) openExisting(rel string, access, attributes, options uint32) (windows.Handle, error) {
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

func (s *platformSys) openDirNoReparse(relDir string) (windows.Handle, error) {
	return s.openExisting(relDir, windows.GENERIC_READ|windows.FILE_LIST_DIRECTORY,
		windows.FILE_ATTRIBUTE_DIRECTORY, windows.FILE_DIRECTORY_FILE)
}

func (s *platformSys) renameNode(oldRel, newRel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.degraded {
		return s.root.Rename(oldRel, newRel)
	}

	handle, err := s.openExisting(oldRel, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE, 0, 0)
	if err != nil {
		return err
	}
	defer closeHandle(handle)

	// Ensure the source node is already owned. Stamping in place after rename is
	// rejected because it leaves a window where unstamped content is published
	// and cannot revoke descriptors already open on the source.
	if !s.degraded {
		uid, gid, mode, err := ntQueryLxEa(handle)
		if err != nil || uid != 0 || gid != 0 || (mode != stampedFileMode() && mode != 040700) {
			return ErrNotOwned
		}
	}

	// Resolve the destination parent directory handle safely without following reparse points.
	dirPart := filepath.Dir(newRel)
	leafPart := filepath.Base(newRel)

	targetParentHandle := s.rootHandle
	if dirPart != "." && dirPart != "" {
		parentH, err := s.openDirNoReparse(dirPart)
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

	nameBytesLen := (len(leaf16) - 1) * 2
	infoSize := unsafe.Sizeof(fileRenameInfoStruct{}) + uintptr(nameBytesLen) - 2 //#nosec G115 // byte length of a short relative path; far below uintptr range.

	buf := make([]byte, infoSize)
	info := (*fileRenameInfoStruct)(unsafe.Pointer(&buf[0])) //#nosec G103 // reinterpreting the rename-info buffer as its documented header; the buffer is sized to hold it.
	info.RootDirectory = targetParentHandle
	info.FileNameLength = uint32(nameBytesLen) //#nosec G115 // rename target byte length; a short relative path, always fits in 32 bits.

	copy((*[1 << 20]byte)(unsafe.Pointer(&info.FileName[0]))[:nameBytesLen], (*[1 << 20]byte)(unsafe.Pointer(&leaf16[0]))[:nameBytesLen]) //#nosec G103 // fixed-size overlay over the rename-info buffer, only ever sliced to the real name length.

	// POSIX semantics is what allows the replacement to happen while readers still
	// hold the destination open, which is the whole point of publishing by rename:
	// without it the kernel refuses the rename outright, even for a reader that
	// shares deletion. Filesystems that predate the information class reject it, so
	// fall back to the original one, where an open destination is simply refused.
	var iosbSet windows.IO_STATUS_BLOCK
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	errSet := windows.NtSetInformationFile(
		handle,
		&iosbSet,
		&buf[0],
		uint32(infoSize), //#nosec G115 // infoSize is small and fits in 32 bits.
		fileRenameInformationEx,
	)

	if isUnsupportedInfoClass(errSet) {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS
		errSet = windows.NtSetInformationFile(
			handle,
			&iosbSet,
			&buf[0],
			uint32(infoSize), //#nosec G115 // infoSize is small and fits in 32 bits.
			windows.FileRenameInformation,
		)
	}

	return mapNtStatus(errSet)
}

// mapNtStatus translates an NT status into a Go error, recognising the
// reparse-blocked signal a rooted, OBJ_DONT_REPARSE syscall produces when a
// symlink component crosses the custodian root.
func mapNtStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.STATUS_STOPPED_ON_SYMLINK) || errors.Is(err, windows.STATUS_REPARSE_POINT_ENCOUNTERED) {
		return ErrPathEscapes
	}
	var ntstatus windows.NTStatus
	if errors.As(err, &ntstatus) {
		return ntstatus.Errno()
	}
	return err
}

// closeHandle closes a Windows handle and discards its error, used in cleanup paths where propagation is not useful.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
}

// ntQueryLxEa reads the $LXUID, $LXGID and $LXMOD extended attributes of the
// node behind h.
func ntQueryLxEa(h windows.Handle) (uid, gid, mode uint32, err error) {
	var iosb windows.IO_STATUS_BLOCK
	buf := make([]byte, 2048)
	if err := windows.NtQueryEaFile(h, &iosb, &buf[0], uint32(len(buf)) /* #nosec G115 */, false, nil, 0, nil, true); err != nil {
		var status windows.NTStatus
		if errors.As(err, &status) {
			return 0, 0, 0, status.Errno()
		}
		return 0, 0, 0, err
	}

	eas, err := winio.DecodeExtendedAttributes(buf[:iosb.Information])
	if err != nil {
		return 0, 0, 0, err
	}

	var found struct{ uid, gid, mode bool }
	for _, ea := range eas {
		if len(ea.Value) == 4 {
			val := binary.LittleEndian.Uint32(ea.Value)
			switch ea.Name {
			case "$LXUID":
				uid, found.uid = val, true
			case "$LXGID":
				gid, found.gid = val, true
			case "$LXMOD":
				mode, found.mode = val, true
			}
		}
	}

	if !found.uid || !found.gid || !found.mode {
		return 0, 0, 0, nil
	}
	return uid, gid, mode, nil
}

func encodeLxEa(uid, gid uint32, mode uint32) ([]byte, error) {
	var uidBytes, gidBytes, modeBytes [4]byte
	binary.LittleEndian.PutUint32(uidBytes[:], uid)
	binary.LittleEndian.PutUint32(gidBytes[:], gid)
	binary.LittleEndian.PutUint32(modeBytes[:], mode)

	eas := []winio.ExtendedAttribute{
		{Name: "$LXUID", Value: uidBytes[:]},
		{Name: "$LXGID", Value: gidBytes[:]},
		{Name: "$LXMOD", Value: modeBytes[:]},
	}
	return winio.EncodeExtendedAttributes(eas)
}

// isUnsupportedInfoClass reports whether err says the filesystem does not implement
// the information class at all, as opposed to refusing this particular operation.
// Only then is retrying with an older class worthwhile.
func isUnsupportedInfoClass(err error) bool {
	return errors.Is(err, windows.STATUS_INVALID_PARAMETER) ||
		errors.Is(err, windows.STATUS_NOT_SUPPORTED) ||
		errors.Is(err, windows.STATUS_INVALID_INFO_CLASS) ||
		errors.Is(err, windows.STATUS_NOT_IMPLEMENTED)
}

func fallbackCreate(root *os.Root, rel string, isDir bool) error {
	if isDir {
		return root.Mkdir(rel, DirMode)
	}
	f, err := root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
	if err != nil {
		return err
	}
	return f.Close()
}

// stampedFileMode returns the Extended Attribute file mode including the file type bits.
func stampedFileMode() uint32 {
	return 0100000 | uint32(FileMode)
}

// relativeAttributes names rel underneath root. Every NT open in this file goes through
// it, so a call site that forgets OBJ_DONT_REPARSE — and would follow a reparse point
// planted inside the sub-tree — cannot be written in the first place.
func relativeAttributes(root windows.Handle, rel string) (*windows.OBJECT_ATTRIBUTES, error) {
	name, err := windows.NewNTUnicodeString(rel)
	if err != nil {
		return nil, err
	}

	return &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})), //#nosec G115 // the size of a fixed struct always fits in 32 bits.
		RootDirectory: root,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}, nil
}
