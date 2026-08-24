//go:build !windows

package daemon

import (
	"fmt"
	"io/fs"
	"syscall"
)

// ownershipOf extracts the UID and GID of the file described by info.
func ownershipOf(info fs.FileInfo) (uid, gid uint32, err error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected stat type %T", info.Sys())
	}

	return stat.Uid, stat.Gid, nil
}
