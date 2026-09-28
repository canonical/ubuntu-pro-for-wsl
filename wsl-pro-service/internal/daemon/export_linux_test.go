package daemon

import (
	"io/fs"

	"golang.org/x/sys/unix"
)

// FileInfoFromStat exports fileInfoFromStat for testing.
func FileInfoFromStat(name string, stat *unix.Stat_t) fs.FileInfo {
	return fileInfoFromStat(name, stat)
}

// NewOpenat2RootForTest constructs an openat2Root with a specified fd for testing edge cases.
func NewOpenat2RootForTest(fd int, path string) RootFs {
	return &openat2Root{fd: fd, path: path}
}
