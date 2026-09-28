package testutils

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
)

func init() {
	// Filesystem types that appear on developer machines and CI runners, so the
	// real root-open path can run there. Production only ever allows the WSL2
	// Windows drive projection: 9p and virtiofs.
	//
	// These must be the fstype names as reported by /proc/self/mountinfo, i.e. the
	// kernel's registered file_system_type names, which do not always match the
	// colloquial filesystem name: Docker's overlayfs reports "overlay".
	daemon.AllowFSNames(
		"tmpfs",
		"ext4",
		"btrfs",
		"xfs",
		"overlay",
		"zfs",
	)
	//nolint:gosec // UID and GID fit in uint32 on POSIX systems.
	daemon.AllowTestOwner(uint32(os.Getuid()), uint32(os.Getgid()))
}
