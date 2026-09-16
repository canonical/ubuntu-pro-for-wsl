// Test-only seams shared by both platforms. This file is compiled by `go test` and is
// never part of the production build, so nothing it exposes exists in a shipped binary.
// It is the sanctioned way to give the external test package reach into the package
// internals without widening the production API (see docs/internal/go-standards.md).

package securefiles
