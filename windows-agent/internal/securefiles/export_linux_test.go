//go:build linux

// Test-only seams. This file is compiled by `go test` and is never part of the
// production build, so nothing it exposes exists in a shipped binary. It is the
// sanctioned way to give the external test package reach into the platform
// internals without widening the production API (see docs/internal/go-standards.md).

package securefiles

// OpenWithXattrs opens a custodian over the given xattr surface, so a filesystem that
// refuses or mishandles extended attributes can be stood in for.
func OpenWithXattrs(basePath string, xattr xattrCalls) (*Custodian, error) {
	return open(basePath, func(p string) (*platformSys, error) { return newPlatformSysWith(p, xattr) })
}

// FailXattr replaces the xattr surface of an already-open custodian, for the cases where
// the failure has to begin after the sub-tree has been seeded.
func (c *Custodian) FailXattr(xattr xattrCalls) {
	if c.sys == nil {
		return
	}
	c.sys.mu.Lock()
	defer c.sys.mu.Unlock()
	c.sys.xattr = xattr
}

// RealXattrCalls exposes the production xattr surface so tests can override one call
// and keep the rest genuine.
func RealXattrCalls() xattrCalls { return realXattrCalls() }
