//go:build windows

package securefiles

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var (
	modntdll                 = windows.NewLazySystemDLL("ntdll.dll")
	procNtCreateFile         = modntdll.NewProc("NtCreateFile")
	procNtSetInformationFile = modntdll.NewProc("NtSetInformationFile")
	procNtSetEaFile          = modntdll.NewProc("NtSetEaFile")
	procNtQueryEaFile        = modntdll.NewProc("NtQueryEaFile")
)

const (
	fileCreate                = 0x00000002
	fileOpen                  = 0x00000001
	fileOpenIf                = 0x00000003
	fileDirectoryFile         = 0x00000001
	fileNonDirectoryFile      = 0x00000040
	fileSynchronousIoNonAlert = 0x00000020

	fileAddFile           = 0x0002
	fileAddSubdirectory   = 0x0004
	fileRenameInformation = 10
)

type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

type objectAttributes struct {
	Length                   uint32
	RootDirectory            windows.Handle
	ObjectName               *unicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

type fileRenameInfoStruct struct {
	ReplaceIfExists uint8
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

type fileFullEaInformation struct {
	NextEntryOffset uint32
	Flags           uint8
	EaNameLength    uint8
	EaValueLength   uint16
	EaName          [1]byte
}

// testNtSetEaFileResult, when non-nil, overrides the return value of NtSetEaFile in tests.
var testNtSetEaFileResult *uint32

// testNtCreateFileResult, when non-nil, overrides the return value of NtCreateFile
// in createNode during tests, so creation failure paths can be exercised without
// sabotaging the root the custodian was opened on.
var testNtCreateFileResult *uint32

// mapNtStatus translates an NT status into a Go error, recognising the
// reparse-blocked signal a rooted, OBJ_DONT_REPARSE syscall produces when a
// symlink component crosses the custodian root.
func mapNtStatus(r1 uintptr) error {
	if windows.NTStatus(r1) == windows.STATUS_STOPPED_ON_SYMLINK { //#nosec G115 // NTSTATUS codes are 32-bit values.
		return ErrPathEscapes
	}
	return windows.NTStatus(r1).Errno() //#nosec G115 // NTSTATUS codes are 32-bit values.
}

type platformSys struct {
	mu         sync.Mutex
	root       *os.Root
	rootFile   *os.File
	rootHandle windows.Handle
	degraded   bool
	mockOwned  *bool
}

// closeHandle closes a Windows handle and discards its error, used in cleanup paths where propagation is not useful.
func closeHandle(h windows.Handle) {
	_ = windows.CloseHandle(h)
}

func newPlatformSys(basePath string) (*platformSys, error) {
	s := &platformSys{
		rootHandle: windows.InvalidHandle,
	}

	_ = s.ensureRoot(basePath)

	return s, nil
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
		windows.GENERIC_READ|fileAddFile|fileAddSubdirectory|windows.FILE_LIST_DIRECTORY,
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

	baseName16, err := windows.UTF16FromString(baseName)
	if err != nil {
		return err
	}

	uString := unicodeString{
		Length:        uint16((len(baseName16) - 1) * 2), //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		MaximumLength: uint16(len(baseName16) * 2),       //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		Buffer:        &baseName16[0],
	}

	oa := objectAttributes{
		Length:        uint32(unsafe.Sizeof(objectAttributes{})),
		RootDirectory: parentHandle,
		ObjectName:    &uString,
		Attributes:    0x00000040,
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE | windows.FILE_WRITE_EA)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_DIRECTORY)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createOptions := uint32(fileSynchronousIoNonAlert | fileDirectoryFile)

	disposition := uint32(fileOpenIf)

	r1, _, _ := procNtCreateFile.Call(
		uintptr(unsafe.Pointer(&handle)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(&oa)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&iosb)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		0,
		uintptr(fileAttributes),
		uintptr(shareAccess),
		uintptr(disposition),
		uintptr(createOptions),
		uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(eaBuf)),
	)

	if r1 != 0 {
		s.degraded = true
		return os.MkdirAll(basePath, DirMode)
	}

	// If opening a pre-existing root directory (iosb.Information == 1 -> FILE_OPENED),
	// NtCreateFile does not apply the eaBuf parameter. Stamp EA via NtSetEaFile.
	if iosb.Information == 1 /* FILE_OPENED */ {
		var iosbSet windows.IO_STATUS_BLOCK
		var r1Set uintptr
		if testNtSetEaFileResult != nil {
			r1Set = uintptr(*testNtSetEaFileResult)
		} else {
			r1Set, _, _ = procNtSetEaFile.Call(
				uintptr(handle),
				uintptr(unsafe.Pointer(&iosbSet)),  //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
				uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
				uintptr(len(eaBuf)),
			)
		}
		if r1Set != 0 {
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
// values the custodian writes. On a degraded filesystem there are no extended
// attributes, so it falls back to the caller's filename recognition and never
// deletes extra nodes.
func (s *platformSys) isOwned(rel string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mockOwned != nil {
		return *s.mockOwned, nil
	}
	if s.degraded {
		return true, nil
	}

	rel16, err := windows.UTF16FromString(rel)
	if err != nil {
		return false, err
	}
	uString := unicodeString{
		Length:        uint16((len(rel16) - 1) * 2), //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		MaximumLength: uint16(len(rel16) * 2),       //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		Buffer:        &rel16[0],
	}
	oa := objectAttributes{
		Length:        uint32(unsafe.Sizeof(objectAttributes{})),
		RootDirectory: s.rootHandle,
		ObjectName:    &uString,
		Attributes:    0x00000040,
	}

	var iosb windows.IO_STATUS_BLOCK
	var h windows.Handle
	r1, _, _ := procNtCreateFile.Call(
		uintptr(unsafe.Pointer(&h)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(windows.GENERIC_READ|windows.SYNCHRONIZE),
		uintptr(unsafe.Pointer(&oa)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&iosb)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		0,
		0,
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		uintptr(fileOpen),
		uintptr(fileSynchronousIoNonAlert|fileNonDirectoryFile),
		0,
		0,
	)
	if r1 != 0 {
		if windows.NTStatus(r1) == windows.STATUS_STOPPED_ON_SYMLINK { //#nosec G115 // NTSTATUS codes are 32-bit values.
			return false, ErrPathEscapes
		}
		return false, windows.NTStatus(r1).Errno() //#nosec G115 // NTSTATUS codes are 32-bit values.
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

// ntQueryLxEa reads the $LXUID, $LXGID and $LXMOD extended attributes of the
// node behind h.
func ntQueryLxEa(h windows.Handle) (uid, gid, mode uint32, err error) {
	var iosb windows.IO_STATUS_BLOCK
	buf := make([]byte, 2048)
	r1, _, _ := procNtQueryEaFile.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&iosb)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&buf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(buf)),
		0, // ReturnSingleEntry = FALSE
		0,
		0,
		0,
		1, // RestartScan = TRUE
	)
	if r1 != 0 {
		return 0, 0, 0, windows.NTStatus(r1).Errno() //#nosec G115 // NTSTATUS codes are 32-bit values.
	}

	var found struct{ uid, gid, mode bool }
	offset := uint32(0)
	for {
		entry := (*fileFullEaInformation)(unsafe.Pointer(&buf[offset])) //#nosec G103 // reinterpreting the kernel-filled EA buffer as its documented header; reads stay within the buffer.
		nameBytes := buf[offset+8 : offset+8+uint32(entry.EaNameLength)]
		valOffset := offset + 8 + uint32(entry.EaNameLength) + 1
		if entry.EaValueLength == 4 {
			val := binary.LittleEndian.Uint32(buf[valOffset : valOffset+4])
			switch string(nameBytes) {
			case "$LXUID":
				uid, found.uid = val, true
			case "$LXGID":
				gid, found.gid = val, true
			case "$LXMOD":
				mode, found.mode = val, true
			}
		}
		if entry.NextEntryOffset == 0 {
			break
		}
		offset += entry.NextEntryOffset
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

	path16, err := windows.UTF16FromString(relativePath)
	if err != nil {
		return err
	}

	uString := unicodeString{
		Length:        uint16((len(path16) - 1) * 2), //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		MaximumLength: uint16(len(path16) * 2),       //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		Buffer:        &path16[0],
	}

	oa := objectAttributes{
		Length:        uint32(unsafe.Sizeof(objectAttributes{})),
		RootDirectory: s.rootHandle,
		ObjectName:    &uString,
		Attributes:    0x00000040,
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE)
	fileAttributes := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createDisposition := uint32(fileCreate)
	createOptions := uint32(fileSynchronousIoNonAlert)

	if isDir {
		createOptions |= fileDirectoryFile
		fileAttributes = windows.FILE_ATTRIBUTE_DIRECTORY
	} else {
		createOptions |= fileNonDirectoryFile
	}

	var r1 uintptr
	if testNtCreateFileResult != nil {
		r1 = uintptr(*testNtCreateFileResult)
	} else {
		r1, _, _ = procNtCreateFile.Call(
			uintptr(unsafe.Pointer(&handle)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			uintptr(desiredAccess),
			uintptr(unsafe.Pointer(&oa)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			uintptr(unsafe.Pointer(&iosb)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			0,
			uintptr(fileAttributes),
			uintptr(shareAccess),
			uintptr(createDisposition),
			uintptr(createOptions),
			uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			uintptr(len(eaBuf)),
		)
	}

	if r1 != 0 {
		ntStatus := windows.NTStatus(r1) //#nosec G115 // NTSTATUS codes are 32-bit values.
		if ntStatus == windows.STATUS_STOPPED_ON_SYMLINK {
			return ErrPathEscapes
		}
		if ntStatus == windows.STATUS_EAS_NOT_SUPPORTED || ntStatus == windows.STATUS_NOT_SUPPORTED || ntStatus == windows.STATUS_INVALID_PARAMETER {
			s.degraded = true
			return fallbackCreate(s.root, relativePath, isDir)
		}
		return ntStatus.Errno()
	}

	closeHandle(handle)
	return nil
}

func (s *platformSys) renameNode(oldRel, newRel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.degraded {
		return s.root.Rename(oldRel, newRel)
	}

	oldRel16, err := windows.UTF16FromString(oldRel)
	if err != nil {
		return err
	}

	uString := unicodeString{
		Length:        uint16((len(oldRel16) - 1) * 2), //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		MaximumLength: uint16(len(oldRel16) * 2),       //#nosec G115 // UNICODE_STRING length of a short relative path; always fits in 16 bits.
		Buffer:        &oldRel16[0],
	}

	oa := objectAttributes{
		Length:        uint32(unsafe.Sizeof(objectAttributes{})),
		RootDirectory: s.rootHandle,
		ObjectName:    &uString,
		Attributes:    0x00000040,
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)

	r1, _, _ := procNtCreateFile.Call(
		uintptr(unsafe.Pointer(&handle)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(&oa)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&iosb)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		0,
		0,
		uintptr(shareAccess),
		uintptr(fileOpen),
		uintptr(fileSynchronousIoNonAlert),
		0,
		0,
	)
	if r1 != 0 {
		return mapNtStatus(r1)
	}
	defer closeHandle(handle)

	newRel16, err := windows.UTF16FromString(newRel)
	if err != nil {
		return err
	}

	nameBytesLen := (len(newRel16) - 1) * 2
	infoSize := unsafe.Sizeof(fileRenameInfoStruct{}) + uintptr(nameBytesLen) - 2 //#nosec G115 // byte length of a short relative path; far below uintptr range.

	buf := make([]byte, infoSize)
	info := (*fileRenameInfoStruct)(unsafe.Pointer(&buf[0])) //#nosec G103 // reinterpreting the rename-info buffer as its documented header; the buffer is sized to hold it.
	info.ReplaceIfExists = 1
	info.RootDirectory = s.rootHandle
	info.FileNameLength = uint32(nameBytesLen) //#nosec G115 // rename target byte length; a short relative path, always fits in 32 bits.

	copy((*[1 << 20]byte)(unsafe.Pointer(&info.FileName[0]))[:nameBytesLen], (*[1 << 20]byte)(unsafe.Pointer(&newRel16[0]))[:nameBytesLen]) //#nosec G103 // fixed-size overlay over the rename-info buffer, only ever sliced to the real name length.

	var iosbSet windows.IO_STATUS_BLOCK
	r1Set, _, _ := procNtSetInformationFile.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&iosbSet)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&buf[0])),  //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		infoSize,
		uintptr(fileRenameInformation),
	)

	for range 100 {
		if r1Set == 0 {
			break
		}
		status := windows.NTStatus(r1Set) //#nosec G115 // NTSTATUS codes are 32-bit values.
		if status == windows.STATUS_STOPPED_ON_SYMLINK {
			return ErrPathEscapes
		}
		if status != windows.STATUS_ACCESS_DENIED && status != windows.STATUS_SHARING_VIOLATION {
			return status.Errno()
		}
		s.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		s.mu.Lock()
		r1Set, _, _ = procNtSetInformationFile.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&iosbSet)), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			uintptr(unsafe.Pointer(&buf[0])),  //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
			infoSize,
			uintptr(fileRenameInformation),
		)
	}

	if r1Set != 0 {
		return windows.NTStatus(r1Set).Errno() //#nosec G115 // NTSTATUS codes are 32-bit values.
	}

	// Stamp EA attributes on the renamed target node through the open handle.
	eaBuf, err := encodeLxEa(0, 0, stampedFileMode())
	if err != nil {
		return err
	}

	var iosbEa windows.IO_STATUS_BLOCK
	r1Ea, _, _ := procNtSetEaFile.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&iosbEa)),   //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(unsafe.Pointer(&eaBuf[0])), //#nosec G103 // NT syscall argument: pointer to live Go memory; the call is synchronous and kernel writes stay within the value.
		uintptr(len(eaBuf)),
	)
	if r1Ea != 0 {
		return windows.NTStatus(r1Ea).Errno() //#nosec G115 // NTSTATUS codes are 32-bit values.
	}

	return nil
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
