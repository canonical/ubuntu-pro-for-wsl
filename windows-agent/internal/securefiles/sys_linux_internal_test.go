//go:build linux

// Unit tests for the Linux watermark internals: the error classifiers that
// distinguish "watermark missing" (not owned) from "xattrs unsupported"
// (degraded, fail-open), and the degraded-mode transitions driven through the
// swappable syscall hooks (mirroring sys_windows.go's testNtSetEaFileResult).

package securefiles

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestXattrErrorClassification(t *testing.T) {
	t.Parallel()

	require.True(t, isXattrUnsupported(unix.ENOTSUP))
	require.True(t, isXattrUnsupported(unix.EOPNOTSUPP))
	require.False(t, isXattrUnsupported(unix.ENODATA))
	require.False(t, isXattrUnsupported(nil))

	require.True(t, isXattrMissing(unix.ENODATA))
	require.False(t, isXattrMissing(unix.ENOTSUP))
	require.False(t, isXattrMissing(nil))
}

// TestXattrDegradedTransitions drives the failure paths of the xattr watermark
// by swapping the syscall hooks. It is intentionally not parallel: the hooks
// are package-level state.
func TestXattrDegradedTransitions(t *testing.T) {
	testCases := map[string]struct {
		// setErr and getErr make the respective syscall fail with the given error.
		setErr error
		getErr error

		// precreate writes a stamped file before enabling the hooks.
		precreate bool

		// op is the operation under test: "write" or "isowned".
		op string

		wantErr      bool
		wantOwned    bool
		wantDegraded bool
	}{
		"write degrades and fails open when xattrs are unsupported": {
			setErr:       unix.ENOTSUP,
			op:           "write",
			wantOwned:    true,
			wantDegraded: true,
		},
		"write fails when stamping fails for another reason": {
			setErr:  unix.EPERM,
			op:      "write",
			wantErr: true,
		},
		"isOwned degrades and reports owned when xattrs are unsupported": {
			getErr:       unix.ENOTSUP,
			precreate:    true,
			op:           "isowned",
			wantOwned:    true,
			wantDegraded: true,
		},
		"isOwned fails when reading the watermark fails for another reason": {
			getErr:    unix.EPERM,
			precreate: true,
			op:        "isowned",
			wantErr:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			c, err := Open(dir)
			require.NoError(t, err, "Setup: could not open custodian")
			defer func() { _ = c.Close() }()

			if tc.precreate {
				require.NoError(t, c.WriteFile("f.txt", []byte("x")), "Setup: could not write file")
			}

			origSet, origGet := fsetxattr, fgetxattr
			t.Cleanup(func() { fsetxattr, fgetxattr = origSet, origGet })
			if tc.setErr != nil {
				fsetxattr = func(int, string, []byte, int) error { return tc.setErr }
			}
			if tc.getErr != nil {
				fgetxattr = func(int, string, []byte) (int, error) { return 0, tc.getErr }
			}

			var opErr error
			owned := false
			switch tc.op {
			case "write":
				opErr = c.WriteFile("f.txt", []byte("x"))
				if opErr == nil {
					owned, opErr = c.IsOwned("f.txt")
				}
			case "isowned":
				owned, opErr = c.IsOwned("f.txt")
			default:
				t.Fatalf("unknown op %q", tc.op)
			}

			if tc.wantErr {
				require.Error(t, opErr)
			} else {
				require.NoError(t, opErr)
			}
			require.Equal(t, tc.wantOwned, owned)
			require.Equal(t, tc.wantDegraded, c.IsDegraded())
		})
	}
}
