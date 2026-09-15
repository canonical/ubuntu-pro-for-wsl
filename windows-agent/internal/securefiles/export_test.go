// Test-only seams shared by both platforms. This file is compiled by `go test` and is
// never part of the production build, so nothing it exposes exists in a shipped binary.
// It is the sanctioned way to give the external test package reach into the package
// internals without widening the production API (see docs/internal/go-standards.md).

package securefiles

import "errors"

// ErrForcedDegradation is the cause SetDegraded records, so that a test can tell a
// degradation it forced from one the code discovered for itself.
var ErrForcedDegradation = errors.New("the filesystem was made to refuse the watermark by a test")

// SetDegraded forces the custodian's whole tree into degraded mode, standing in for a
// filesystem that cannot carry the watermark. Tests that must degrade a custodian *after*
// seeding it use this: the data has to predate the loss of the attribute, exactly as it
// does when a healthy profile is later moved to a filesystem without extended attributes.
func (c *Custodian) SetDegraded(degraded bool) {
	if c.sys == nil {
		return
	}
	c.sys.deg.mu.Lock()
	defer c.sys.deg.mu.Unlock()

	if !degraded {
		c.sys.deg.first = nil
		return
	}
	c.sys.deg.first = ErrForcedDegradation
}
