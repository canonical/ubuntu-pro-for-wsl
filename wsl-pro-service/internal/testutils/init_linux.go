package testutils

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"golang.org/x/sys/unix"
)

// Standard Linux filesystems used across developer environments and CI runners.
const (
	magicZFS = 0x2fc12fc1
)

func init() {
	daemon.AllowFSTypes(
		unix.TMPFS_MAGIC,
		unix.EXT4_SUPER_MAGIC,
		unix.BTRFS_SUPER_MAGIC,
		unix.XFS_SUPER_MAGIC,
		unix.OVERLAYFS_SUPER_MAGIC,
		magicZFS,
	)
	//nolint:gosec // UID and GID fit in uint32 on POSIX systems.
	daemon.AllowTestOwner(uint32(os.Getuid()), uint32(os.Getgid()))
}
