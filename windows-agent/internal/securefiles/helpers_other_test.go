//go:build !windows

package securefiles_test

import "testing"

// holdReparsePointOpen has no non-Windows form, and not for lack of a way to open
// the link itself: unix.Open answers O_PATH|O_NOFOLLOW (measured: the descriptor
// is the link, never the target). It is useless for the cannot-remove case because
// POSIX has no delete-sharing: an open descriptor, however opened, never blocks an
// unlink, so the removal the case needs refused is refused on those platforms by
// the tree root's write permission instead. It must never be reached.
func holdReparsePointOpen(t *testing.T, _ string) {
	t.Helper()
	t.Fatal("holding a link open has no non-Windows form")
}
