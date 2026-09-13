//go:build windows

package securefiles

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
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

type fileRenameInfoStruct struct {
	ReplaceIfExists uint8
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

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

	uString, err := windows.NewNTUnicodeString(baseName)
	if err != nil {
		return err
	}

	oa := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parentHandle,
		ObjectName:    uString,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
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
		&oa,
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

	uString, err := windows.NewNTUnicodeString(rel)
	if err != nil {
		return false, err
	}
	oa := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: s.rootHandle,
		ObjectName:    uString,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}

	var iosb windows.IO_STATUS_BLOCK
	var h windows.Handle
	err = windows.NtCreateFile(
		&h,
		windows.GENERIC_READ|windows.SYNCHRONIZE,
		&oa,
		&iosb,
		nil,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_NON_DIRECTORY_FILE,
		0,
		0,
	)
	if err != nil {
		if errors.Is(err, windows.STATUS_STOPPED_ON_SYMLINK) {
			return false, ErrPathEscapes
		}
		var status windows.NTStatus
		if errors.As(err, &status) {
			return false, status.Errno()
		}
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

	uString, err := windows.NewNTUnicodeString(relativePath)
	if err != nil {
		return err
	}

	oa := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: s.rootHandle,
		ObjectName:    uString,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
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
			&oa,
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
		if errors.Is(ntErr, windows.STATUS_STOPPED_ON_SYMLINK) {
			return ErrPathEscapes
		}
		if errors.Is(ntErr, windows.STATUS_EAS_NOT_SUPPORTED) || errors.Is(ntErr, windows.STATUS_NOT_SUPPORTED) || errors.Is(ntErr, windows.STATUS_INVALID_PARAMETER) {
			s.degraded = true
			return fallbackCreate(s.root, relativePath, isDir)
		}
		var status windows.NTStatus
		if errors.As(ntErr, &status) {
			return status.Errno()
		}
		return ntErr
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

	oldUString, err := windows.NewNTUnicodeString(oldRel)
	if err != nil {
		return err
	}

	oa := windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: s.rootHandle,
		ObjectName:    oldUString,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}

	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle

	desiredAccess := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE | windows.SYNCHRONIZE)
	shareAccess := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)

	err = windows.NtCreateFile(
		&handle,
		desiredAccess,
		&oa,
		&iosb,
		nil,
		0,
		shareAccess,
		windows.FILE_OPEN,
		windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return mapNtStatus(err)
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
	errSet := windows.NtSetInformationFile(
		handle,
		&iosbSet,
		&buf[0],
		uint32(infoSize), //#nosec G115 // infoSize is small and fits in 32 bits.
		windows.FileRenameInformation,
	)

	for range 100 {
		if errSet == nil {
			break
		}
		if errors.Is(errSet, windows.STATUS_STOPPED_ON_SYMLINK) {
			return ErrPathEscapes
		}
		if !errors.Is(errSet, windows.STATUS_ACCESS_DENIED) && !errors.Is(errSet, windows.STATUS_SHARING_VIOLATION) {
			var status windows.NTStatus
			if errors.As(errSet, &status) {
				return status.Errno()
			}
			return errSet
		}
		s.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		s.mu.Lock()
		errSet = windows.NtSetInformationFile(
			handle,
			&iosbSet,
			&buf[0],
			uint32(infoSize), //#nosec G115 // infoSize is small and fits in 32 bits.
			windows.FileRenameInformation,
		)
	}

	if errSet != nil {
		var status windows.NTStatus
		if errors.As(errSet, &status) {
			return status.Errno()
		}
		return errSet
	}

	// Stamp EA attributes on the renamed target node through the open handle.
	eaBuf, err := encodeLxEa(0, 0, stampedFileMode())
	if err != nil {
		return err
	}

	var iosbEa windows.IO_STATUS_BLOCK
	errEa := windows.NtSetEaFile(
		handle,
		&iosbEa,
		&eaBuf[0],
		uint32(len(eaBuf)), //#nosec G115 // length of small EA buffer; always fits in 32 bits.
	)
	if errEa != nil {
		var status windows.NTStatus
		if errors.As(errEa, &status) {
			return status.Errno()
		}
		return errEa
	}

	return nil
}

// mapNtStatus translates an NT status into a Go error, recognising the
// reparse-blocked signal a rooted, OBJ_DONT_REPARSE syscall produces when a
// symlink component crosses the custodian root.
func mapNtStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.STATUS_STOPPED_ON_SYMLINK) {
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
