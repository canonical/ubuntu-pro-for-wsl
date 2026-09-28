package daemon

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestProductionFSNames pins the production allowlist so the security-critical
// constant cannot drift unnoticed, for instance by someone adding a type meant
// only for tests. AllowFSNames mutates the runtime copy, allowedFSNames, never
// this definition.
func TestProductionFSNames(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"9p", "virtiofs"}, productionFSNames)
}

// TestParseMntID covers the fdinfo parsing used to pin which mount the opened
// root descriptor belongs to. Anything unexpected must fail closed.
func TestParseMntID(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fdinfo  string
		wantID  int
		wantErr string
	}{
		"Parses mnt_id among other fields": {
			fdinfo: "pos:\t0\nflags:\t0100000\nmnt_id:\t31\n",
			wantID: 31,
		},
		"Parses mnt_id on the first line": {
			fdinfo: "mnt_id:\t7\n",
			wantID: 7,
		},
		"Fails when mnt_id is absent": {
			fdinfo:  "pos:\t0\nflags:\t0100000\n",
			wantErr: "no mnt_id in fdinfo",
		},
		"Fails when mnt_id is not a number": {
			fdinfo:  "mnt_id:\tnot-a-number\n",
			wantErr: "invalid mnt_id",
		},
		"Fails on empty fdinfo": {
			fdinfo:  "",
			wantErr: "no mnt_id in fdinfo",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			id, err := parseMntID(tc.fdinfo)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr, "parseMntID should fail closed")
				return
			}
			require.NoError(t, err, "parseMntID should succeed")
			require.Equal(t, tc.wantID, id, "unexpected mount ID")
		})
	}
}

// TestFstypeForMountID covers the mountinfo lookup matching a mount by ID and
// reporting its filesystem type and mount point. Only whole fields are
// inspected, so the octal escapes mountinfo uses inside paths never apply.
func TestFstypeForMountID(t *testing.T) {
	t.Parallel()

	mountinfo := "36 35 98:0 / /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue\n" +
		"31 30 0:32 / /mnt/c rw,relatime - 9p C:\\ rw,dirsync,aname=drvfs\040path=C:\\;mfsymlinks\n" +
		"32 31 0:33 / /home/user/mount rw,nosuid,nodev,relatime user_id=1000 - fuse.sshfs sshfs#user@host rw\n" +
		"38 37 0:39 / /mnt/d rw,relatime - virtiofs D:\\ rw\n"

	testCases := map[string]struct {
		mountinfo   string
		mntID       int
		wantFSType  string
		wantMountAt string
		wantErr     string
	}{
		"Finds the 9p Windows drive mount": {
			mountinfo: mountinfo, mntID: 31,
			wantFSType: "9p", wantMountAt: "/mnt/c",
		},
		"Finds an entry with optional tagged fields before the separator": {
			mountinfo: mountinfo, mntID: 36,
			wantFSType: "ext3", wantMountAt: "/mnt2",
		},
		"Finds a FUSE mount with an attacker-chosen subtype": {
			mountinfo: mountinfo, mntID: 32,
			wantFSType: "fuse.sshfs", wantMountAt: "/home/user/mount",
		},
		"Finds the virtiofs Windows drive mount": {
			mountinfo: mountinfo, mntID: 38,
			wantFSType: "virtiofs", wantMountAt: "/mnt/d",
		},
		"Fails when the mount ID is absent": {
			mountinfo: mountinfo, mntID: 99,
			wantErr: "mount 99 not found in mountinfo",
		},
		"Fails when the matching entry has no separator": {
			mountinfo: "40 39 0:40 / /x rw,relatime\n", mntID: 40,
			wantErr: "malformed mountinfo entry for mount 40",
		},
		"Fails when the separator is the last field": {
			mountinfo: "41 40 0:41 / /y rw,relatime -\n", mntID: 41,
			wantErr: "malformed mountinfo entry for mount 41",
		},
		"Fails when the matching entry is too short": {
			mountinfo: "42 41 0:42\n", mntID: 42,
			wantErr: "malformed mountinfo entry for mount 42",
		},
		"Ignores entries whose first field merely contains the ID": {
			mountinfo: "310 30 0:32 / /other rw,relatime - ext4 /dev/sda1 rw\n" +
				"31 30 0:32 / /mnt/c rw,relatime - 9p C:\\ rw\n",
			mntID:      31,
			wantFSType: "9p", wantMountAt: "/mnt/c",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fstype, mountPoint, err := fstypeForMountID(tc.mountinfo, tc.mntID)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr, "fstypeForMountID should fail closed")
				return
			}
			require.NoError(t, err, "fstypeForMountID should succeed")
			require.Equal(t, tc.wantFSType, fstype, "unexpected filesystem type")
			require.Equal(t, tc.wantMountAt, mountPoint, "unexpected mount point")
		})
	}
}

// TestMountFSType exercises the real /proc/self/fdinfo and /proc/self/mountinfo
// lookup on live descriptors. /proc is used because its filesystem type is
// deterministic across environments; a temp directory is used to assert the
// lookup succeeds for a regular test directory (its exact type depends on the
// runner, which is why testutils extends the allowlist).
func TestMountFSType(t *testing.T) {
	t.Parallel()

	t.Run("Reports the proc filesystem for a descriptor on /proc", func(t *testing.T) {
		t.Parallel()

		fd, err := unix.Open("/proc", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		require.NoError(t, err, "Setup: could not open /proc")
		t.Cleanup(func() { _ = unix.Close(fd) })

		fstype, mountPoint, err := mountFSType(fd)
		require.NoError(t, err, "mountFSType should succeed on /proc")
		require.Equal(t, "proc", fstype, "unexpected filesystem type for /proc")
		require.Equal(t, "/proc", mountPoint, "unexpected mount point for /proc")
	})

	t.Run("Succeeds on a regular test directory", func(t *testing.T) {
		t.Parallel()

		fd, err := unix.Open(t.TempDir(), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		require.NoError(t, err, "Setup: could not open temp directory")
		t.Cleanup(func() { _ = unix.Close(fd) })

		fstype, _, err := mountFSType(fd)
		require.NoError(t, err, "mountFSType should succeed on a test directory")
		require.True(t, allowedFSNames[fstype], "temp directory filesystem %q should be allowed by the test seam", fstype)
	})
}

// TestOpenRootOS_RefusesUntrustedFilesystemReal asserts, with the real
// openRootOS, that a root on a filesystem outside the allowlist is refused even
// though the directory itself is perfectly readable.
func TestOpenRootOS_RefusesUntrustedFilesystemReal(t *testing.T) {
	t.Parallel()

	root, err := openRootOS("/sys/kernel")
	require.Error(t, err, "openRootOS should refuse a root on sysfs")
	require.Nil(t, root, "no root should be returned on refusal")
	require.ErrorContains(t, err, `refusing untrusted filesystem "sysfs"`, "refusal should name the observed filesystem")

	// The refusal must not be mistaken for the agent not having written its files yet.
	require.False(t, os.IsNotExist(err), "refusal must not alias os.ErrNotExist")
}
