//go:build windows

// Test-only seams. This file is compiled by `go test` and is never part of the
// production build, so nothing it exposes exists in a shipped binary. It is the
// sanctioned way to give the external test package reach into the platform
// internals without widening the production API (see docs/internal/go-standards.md).

package securefiles

import "golang.org/x/sys/windows"

// OpenRefusingStamp opens a custodian on a platform whose extended-attribute writes fail
// from the outset, standing in for a volume that refuses them. It drives the transition
// ADR 2.02 governs: stamping refused, degrade loudly, never fail closed.
func OpenRefusingStamp(basePath string, status uint32) (*Custodian, error) {
	return open(basePath, func(p string) (*platformSys, error) {
		nt := realNtCalls()
		nt.setEaFile = func(windows.Handle, []byte) error { return windows.NTStatus(status) }
		return newPlatformSysWith(p, nt)
	})
}

// OpenRefusingCreation opens a custodian whose root creation fails with status, standing
// in for a pre-existing directory whose ACL denies the access the stamp needs. Only a
// filesystem that cannot carry extended attributes may be adopted unstamped (ADR 2.02);
// anything else has to reach the caller.
func OpenRefusingCreation(basePath string, status uint32) (*Custodian, error) {
	return open(basePath, func(p string) (*platformSys, error) {
		nt := realNtCalls()
		nt.createFile = func(*windows.Handle, uint32, *windows.OBJECT_ATTRIBUTES, *windows.IO_STATUS_BLOCK, uint32, uint32, uint32, uint32, []byte) error {
			return windows.NTStatus(status)
		}
		return newPlatformSysWith(p, nt)
	})
}

// FailStamping makes every later extended-attribute write on this custodian fail.
func (c *Custodian) FailStamping(status uint32) {
	if c.sys == nil {
		return
	}
	c.sys.mu.Lock()
	defer c.sys.mu.Unlock()
	c.sys.nt.setEaFile = func(windows.Handle, []byte) error { return windows.NTStatus(status) }
}

// FailCreation makes node creation on this custodian fail with status.
func (c *Custodian) FailCreation(status uint32) {
	if c.sys == nil {
		return
	}
	c.sys.mu.Lock()
	defer c.sys.mu.Unlock()
	c.sys.nt.createFile = func(*windows.Handle, uint32, *windows.OBJECT_ATTRIBUTES, *windows.IO_STATUS_BLOCK, uint32, uint32, uint32, uint32, []byte) error {
		return windows.NTStatus(status)
	}
}

// WithoutRenameInfoEx makes the first rename attempt report the information class as
// unimplemented, as a volume predating FILE_RENAME_INFORMATION_EX does.
func (c *Custodian) WithoutRenameInfoEx() {
	if c.sys == nil {
		return
	}
	c.sys.mu.Lock()
	defer c.sys.mu.Unlock()
	passthrough := c.sys.nt.setInformationFile
	c.sys.nt.setInformationFile = func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, buf []byte, class uint32) error {
		if class == fileRenameInformationEx {
			return windows.STATUS_NOT_IMPLEMENTED
		}
		return passthrough(h, iosb, buf, class)
	}
}
