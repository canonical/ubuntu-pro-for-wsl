//go:build windows

package securefiles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

type platformSys struct {
	mu         sync.Mutex
	nt         ntCalls
	root       *os.Root
	rootFile   *os.File
	rootHandle windows.Handle
	// deg is shared with every sub-custodian derived from this one.
	deg *degradation

	// rootID identifies the directory ensureRoot created and stamped, so that the
	// separate reopen in setRoot can be checked against it. Unset for sub-custodians,
	// which are derived from a parent handle and never resolved by path.
	rootID      fileIdentity
	rootIDKnown bool
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

// ntCalls is the NT surface the stamping path uses. It is a field of platformSys
// rather than a set of package-level variables so that there is no process-wide switch
// an attacker could flip to turn stamping off for every custodian at once: it is set
// once at construction and never reassigned. Production wires realNtCalls; the tests in
// this package wire a fake to reproduce the two conditions only a filesystem can create
// — extended attributes refused, and a rename information class the volume does not
// implement.
type ntCalls struct {
	setEaFile          func(h windows.Handle, ea []byte) error
	createFile         func(handle *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, attrs, share, disp, opts uint32, ea []byte) error
	setInformationFile func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error
	identity           func(h windows.Handle) (fileIdentity, bool)
}

// fileIdentity is a node's identity as the filesystem reports it: the volume it lives on
// and its file ID. It is what distinguishes the directory the custodian created from
// whatever its path resolves to a moment later.
type fileIdentity struct {
	volume uint64
	file   [16]byte
}

// fileIDInfo mirrors FILE_ID_INFO, which golang.org/x/sys/windows does not declare.
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
		createFile: func(handle *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, attrs, share, disp, opts uint32, ea []byte) error {
			return windows.NtCreateFile(
				handle, access, oa, iosb, nil, attrs, share, disp, opts,
				uintptr(unsafe.Pointer(&ea[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
				uint32(len(ea)),                 //#nosec G115 // length of a small EA buffer; always fits in 32 bits.
			)
		},
		setInformationFile: func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error {
			return windows.NtSetInformationFile(h, iosb, &buf[0], uint32(len(buf)), class) //#nosec G115 // rename-info buffer; always fits in 32 bits.
		},
		identity: identityOf,
	}
}

func newPlatformSys(basePath string) (*platformSys, error) {
	return newPlatformSysWith(basePath, realNtCalls())
}

// newPlatformSysWith is newPlatformSys with the NT surface supplied rather than assumed.
func newPlatformSysWith(basePath string, nt ntCalls) (*platformSys, error) {
	s := &platformSys{
		rootHandle: windows.InvalidHandle,
		nt:         nt,
		deg:        &degradation{},
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
func newSubPlatformSys(parent *platformSys) *platformSys {
	return &platformSys{
		rootHandle: windows.InvalidHandle,
		deg:        parent.deg,
		nt:         parent.nt,
	}
}

// stampSubdir stamps an already-existing sub-directory in place, when the custodian adopts
// one left by an earlier run. ADR 2.01 requires first-level sub-tree roots to carry the
// stamp: Linux checks a directory's ownership and mode on every operation, so the stamp
// revokes unprivileged creation and deletion inside it. The node is opened relative to the
// root handle without following reparse points, so adoption cannot be redirected outside
// the sub-tree.
func (s *platformSys) stampSubdir(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isDegraded() {
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

	if err := s.nt.setEaFile(handle, eaBuf); err != nil {
		if !eaUnsupported(err) {
			return mapNtStatus(err)
		}
		s.deg.note(fmt.Errorf("could not stamp the sub-directory %s: %w", rel, err))
	}

	return nil
}

// setRoot derives the root directory handle used for relative NtCreateFile
// calls from the os.Root, so EA-stamped creation is rooted at the same
// directory that provides structural containment.
func (s *platformSys) setRoot(root *os.Root) error {
	f, err := root.Open(".")
	if err != nil {
		return err
	}

	// A second, independent resolution of the path ensureRoot already created, stamped
	// and closed. os.Root contains the names opened through it, not its own root, and
	// the parent directory stays writable from inside an instance (ADR 2.01), so between
	// the two resolutions it can be made to point elsewhere. Identity ties them to one node.
	//
	// A filesystem that supplies no identity is conceded once, at the first resolution:
	// nothing was recorded, so there is nothing to compare. Once one has been recorded
	// the concession is spent, and a reopened handle that cannot identify itself is a
	// failed verification rather than an absent one — otherwise a swap onto a filesystem
	// exposing no identity would walk past this check.
	if s.rootIDKnown {
		id, ok := s.nt.identity(windows.Handle(f.Fd()))
		if !ok || id != s.rootID {
			return errors.Join(ErrRootReplaced, f.Close())
		}
	}

	s.root = root
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

	err = s.nt.createFile(&handle, desiredAccess, oa, &iosb, fileAttributes, shareAccess, disposition, createOptions, eaBuf)
	if err != nil {
		// Only a filesystem that cannot carry extended attributes may be adopted unstamped
		// (ADR 2.02). Anything else refusing this call — an ACL, a filter driver, a
		// sharing violation — leaves a directory that could have been stamped, and the
		// fallback hides all of them, because os.MkdirAll succeeds on a directory already
		// there: the custodian would serve an unstamped root reporting nothing wrong.
		//
		// A root behind a junction or symlink takes the same road out, and must:
		// OBJ_DONT_REPARSE is what failed this call rather than follow the link, and the
		// fallback would undo that in one line, leaving setRoot no identity to compare.
		if !eaUnsupported(err) {
			return mapNtStatus(err)
		}

		s.deg.note(fmt.Errorf("could not create the root with its ownership stamp: %w", err))
		return os.MkdirAll(basePath, DirMode)
	}

	// If opening a pre-existing root directory (iosb.Information == 1 -> FILE_OPENED),
	// NtCreateFile does not apply the eaBuf parameter. Stamp EA via NtSetEaFile.
	if iosb.Information == 1 /* FILE_OPENED */ {
		if err := s.nt.setEaFile(handle, eaBuf); err != nil {
			if !eaUnsupported(err) {
				closeHandle(handle)
				return mapNtStatus(err)
			}
			s.deg.note(fmt.Errorf("could not stamp the pre-existing root: %w", err))
		}
	}

	// Record what was created, so the reopen in setRoot can be tied back to it.
	s.rootID, s.rootIDKnown = s.nt.identity(handle)

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
	return s.deg.cause() != nil
}

// isOwned reports whether the node at rel carries the agent's watermark: the
// $LXUID/$LXGID/$LXMOD stamp queried through NtQueryEaFile, for exactly the values the
// custodian writes. It never answers on behalf of a filesystem that cannot carry them:
// there the query fails and ownership is unknowable rather than true. Callers consult
// isDegraded first and decide what an unverifiable sub-tree means, because reading an
// unknowable answer as "ours" or "foreign" is policy, not a fact this predicate has.
func (s *platformSys) isOwned(rel string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *platformSys) createNode(relativePath string, isDir bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := stampedFileMode()
	if isDir {
		mode = uint32(040700)
	}

	if s.isDegraded() {
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

	ntErr := s.nt.createFile(&handle, desiredAccess, oa, &iosb, fileAttributes, shareAccess, createDisposition, createOptions, eaBuf)

	if ntErr != nil {
		// A filesystem that will not take the attribute buffer degrades and falls back;
		// every other failure, a redirected path above all, is reported as it is.
		if eaUnsupported(ntErr) {
			s.deg.note(fmt.Errorf("could not create %s with its ownership stamp: %w", relativePath, ntErr))
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

	if s.isDegraded() {
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
	if !s.isDegraded() {
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
	errSet := s.nt.setInformationFile(handle, &iosbSet, buf, fileRenameInformationEx)

	if isUnsupportedInfoClass(errSet) {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS
		errSet = s.nt.setInformationFile(handle, &iosbSet, buf, windows.FileRenameInformation)
	}

	return mapNtStatus(errSet)
}

// eaUnsupported reports whether a status means the filesystem cannot carry extended
// attributes at all, rather than this write having been refused. Only the first is what
// ADR 2.02 keeps serving through; calling a refusal "degraded" claims the machine cannot
// be secured when something merely stopped us securing it.
//
// Measured against a volume that cannot store them, not assumed: creating a node with an
// attribute buffer answers STATUS_EAS_NOT_SUPPORTED, setting one on a node already there
// answers STATUS_INVALID_DEVICE_REQUEST. Adoption only ever sees the second, and a
// sub-tree left by an earlier run is always adopted.
//
// STATUS_INVALID_PARAMETER is excluded on purpose: no such volume returns it, but our own
// buffer or flags would if they ever became wrong, and absorbing that would fail open on
// every machine at once. A malformed buffer answers STATUS_EA_LIST_INCONSISTENT.
func eaUnsupported(err error) bool {
	return errors.Is(err, windows.STATUS_EAS_NOT_SUPPORTED) ||
		errors.Is(err, windows.STATUS_INVALID_DEVICE_REQUEST) ||
		errors.Is(err, windows.STATUS_NOT_SUPPORTED)
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

// identityOf reports the identity of the node behind h, and whether the filesystem
// supplied a usable one. FileIdInfo is asked first because its 128-bit ID is the real
// one: ReFS reports a narrower derived value through GetFileInformationByHandle, so
// comparing the 64-bit form there is weaker than it looks. A filesystem that supplies no
// identity is not an error; it only means the node cannot be re-identified later.
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

// closeHandle closes a Windows handle and discards its error, used in cleanup paths where propagation is not useful.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
}

// remoteVolume reports whether path lives on a volume this machine does not own, and names
// the kind. A UNC path is classified by its prefix, not GetDriveType: for an unreachable
// server that call blocks on name resolution for seconds and then answers
// DRIVE_NO_ROOT_DIR anyway. Drive letters are cheap, so they go through the API, which is
// what identifies a mapped network drive.
//
// The path must already be resolved: a directory in the profile can be a link to a share,
// where the name says "C:" while the bytes live on a server.
func remoteVolume(path string) (remote bool, kind string) {
	vol := filepath.VolumeName(stripNTPrefix(path))
	if strings.HasPrefix(vol, `\\`) {
		return true, "a UNC path"
	}
	if vol == "" {
		return false, ""
	}

	root, err := windows.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return false, ""
	}
	if windows.GetDriveType(root) == windows.DRIVE_REMOTE {
		return true, "a mapped network drive"
	}

	return false, ""
}

// stripNTPrefix turns the NT forms that GetFinalPathNameByHandle returns into the ones
// filepath understands: \\?\C:\dir stays a drive, and \\?\UNC\server\share becomes the
// UNC path it denotes, so a redirected directory is classified by where it really is.
func stripNTPrefix(path string) string {
	const dos, unc = `\\?\`, `\\?\UNC\`
	if strings.HasPrefix(path, unc) {
		return `\\` + path[len(unc):]
	}
	return strings.TrimPrefix(path, dos)
}

// resolvedBasePath returns the sub-tree root as the filesystem finally names it, following
// any links in the path the custodian was handed. It answers "" when the answer is
// unavailable, which leaves the caller with the name it already had.
func (s *platformSys) resolvedBasePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rootHandle == 0 {
		return ""
	}

	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(s.rootHandle, &buf[0], uint32(len(buf)), 0) //#nosec G115 // fixed-size path buffer; always fits in 32 bits.
	if err != nil || n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
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
