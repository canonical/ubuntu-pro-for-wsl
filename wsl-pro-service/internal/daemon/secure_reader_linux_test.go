package daemon_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestDefaultValidate asserts that the default validation rules accept strictly root-owned
// files and directories (mode 0600 for files, mode 0700 for directories with root UID/GID)
// and reject non-root ownership, permissive modes, or non-regular files.
// This is useful because it ensures administrative configuration files consumed by the daemon
// cannot be modified or replaced by unprivileged users.
func TestDefaultValidate(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		info    fs.FileInfo
		path    string
		wantErr string
	}{
		"Valid directory": {
			info: secureDirInfo("dir"),
			path: "/dir",
		},
		"Valid regular file": {
			info: secureFileInfo("file"),
			path: "/file",
		},
		"Invalid UID on file": {
			info:    mockFileInfo{mode: 0600, sys: &syscall.Stat_t{Uid: 1000, Gid: 0}},
			path:    "/file",
			wantErr: `refused "/file": not strictly owned by root (uid 1000, gid 0)`,
		},
		"Invalid GID on file": {
			info:    mockFileInfo{mode: 0600, sys: &syscall.Stat_t{Uid: 0, Gid: 1000}},
			path:    "/file",
			wantErr: `refused "/file": not strictly owned by root (uid 0, gid 1000)`,
		},
		"Invalid directory mode": {
			info:    mockFileInfo{isDir: true, mode: fs.ModeDir | 0755, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
			path:    "/dir",
			wantErr: `refused directory "/dir": not strictly owned by root (mode 0755)`,
		},
		"Invalid file mode": {
			info:    mockFileInfo{mode: 0644, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
			path:    "/file",
			wantErr: `refused file "/file": not strictly owned by root (mode 0644)`,
		},
		"Non stat_t sys metadata": {
			info:    mockFileInfo{mode: 0600, sys: nil},
			path:    "/file",
			wantErr: `could not obtain ownership metadata for "/file": unexpected stat type <nil>`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := daemon.DefaultValidate(tc.path, tc.info)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr, "DefaultValidate should return expected error")
			} else {
				require.NoError(t, err, "DefaultValidate should succeed on valid path/info")
			}
		})
	}
}

// TestDefaultSecureReader drives the mocked-seam cases: validation logic, walk-loop error
// propagation, and the close invariant. Cases that need real filesystem semantics (a symlink
// rootDir, a missing rootDir) live in TestDefaultSecureReader_RealFS below; cases that need
// root-owned 0600/0700 paths live in TestDefaultSecureReader_RealFS_RefusesNonRootOwnership.
// Confinement of escape paths ("..", absolute) is exercised by
// TestOpenRootOS_ConfinesPathResolution on the real os.Root seam.
func TestDefaultSecureReader(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// root is the mockRootFs the test injects via the openRoot seam. Each case
		// also supplies openRootErr to simulate OpenRoot returning an error.
		root        *mockRootFs
		openRootErr error

		targetPath  string
		wantErr     string
		wantContent string // expected string for the success-path case; empty otherwise
		wantClosed  bool
	}{
		"Valid nested file is read and the root is closed": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"sub":            secureDirInfo("sub"),
					"sub/secret.txt": secureFileInfo("secret.txt"),
				},
				contents: map[string]string{"sub/secret.txt": "hello mock secret"}, //nolint:gosec // test fixture, not a credential
			},
			targetPath:  "sub/secret.txt",
			wantContent: "hello mock secret",
			wantClosed:  true,
		},
		"Rejected when opening the root fails": {
			openRootErr: errors.New("permission denied"),
			targetPath:  "file.txt",
			wantErr:     "could not open root",
		},
		"Rejected when stating a component fails": {
			root:       &mockRootFs{lstatErr: errors.New("disk failure")},
			targetPath: "sub/file.txt",
			wantErr:    "could not stat",
			wantClosed: true,
		},
		"Rejected on symlink file": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"symlink.txt": mockFileInfo{name: "symlink.txt", mode: fs.ModeSymlink | 0777, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
				},
			},
			targetPath: "symlink.txt",
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on intermediate symlink directory": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"symlink_dir": mockFileInfo{name: "symlink_dir", mode: fs.ModeSymlink | 0777, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
				},
			},
			targetPath: "symlink_dir/file.txt",
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on irregular file type": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"pipe": mockFileInfo{name: "pipe", mode: fs.ModeNamedPipe | 0600, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
				},
			},
			targetPath: "pipe",
			wantErr:    "irregular file type",
			wantClosed: true,
		},
		"Rejected when an intermediate directory has insecure permissions": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"sub":          mockFileInfo{name: "sub", isDir: true, mode: fs.ModeDir | 0755, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
					"sub/file.txt": secureFileInfo("file.txt"),
				},
			},
			targetPath: "sub/file.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when the file has insecure permissions": {
			root: &mockRootFs{
				infos: map[string]fs.FileInfo{
					"file.txt": mockFileInfo{name: "file.txt", mode: 0644, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
				},
			},
			targetPath: "file.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when opening the target fails": {
			root: &mockRootFs{
				infos:   map[string]fs.FileInfo{"file.txt": secureFileInfo("file.txt")},
				openErr: errors.New("open error: permission denied"),
			},
			targetPath: "file.txt",
			wantErr:    "could not read",
			wantClosed: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// A fresh tempdir satisfies the inline os.Lstat(rootDir) check: t.TempDir
			// creates a real directory owned by the test user with mode 0700-and-not-a-
			// symlink. The mock openRoot seam then takes over for the root-relative
			// operations the reader would otherwise perform.
			rootDir := t.TempDir()

			openRoot := func(string) (daemon.RootFs, error) {
				if tc.openRootErr != nil {
					return nil, tc.openRootErr
				}
				return tc.root, nil
			}
			reader := daemon.NewDefaultSecureReader(openRoot)
			got, err := reader.ReadFile(rootDir, tc.targetPath)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr, "reader.ReadFile should fail with expected error")
			} else {
				require.NoError(t, err, "reader.ReadFile should succeed on valid input")
				require.Equal(t, tc.wantContent, string(got), "read content does not match expected")
			}

			if tc.wantClosed {
				require.NotNil(t, tc.root, "wantClosed requires root to be set")
				require.True(t, tc.root.closed, "the root must be closed after ReadFile")
			}
		})
	}
}

// TestDefaultSecureReader_RealFS exercises the inline os.Lstat(rootDir) check that runs
// before the openRoot seam. Unprivileged tests cannot reach the success path through the
// reader (validateNode refuses non-root ownership), so we cover the refusal paths here.
func TestDefaultSecureReader_RealFS(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		symlink bool
		missing bool
		notRoot bool

		wantErr string
	}{
		"Refuses a symlink rootDir":             {symlink: true, wantErr: "root is a symlink"},
		"Fails on missing rootDir":              {missing: true, wantErr: "could not stat"},
		"Fails if rootDir is not owned by root": {notRoot: true, wantErr: "not strictly owned by root"},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			require.NoError(t, os.Chmod(rootDir, 0o700), "Setup: failed to adjust target directory mode") //nolint:gosec // test exercises 0700 directory validation

			var filePath string
			if tc.symlink {
				linkParent := t.TempDir()
				link := filepath.Join(linkParent, "link-to-target")
				require.NoError(t, os.Symlink(rootDir, link), "Setup: failed to create root symlink")
				rootDir = link
			}
			if tc.missing {
				rootDir = filepath.Join(t.TempDir(), "does-not-exist")
			}
			if tc.notRoot {
				filePath = filepath.Join(rootDir, "anything.txt")
				require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o600), "Setup: failed to write test file")
			}

			reader := daemon.NewDefaultSecureReader(nil)
			_, err := reader.ReadFile(rootDir, filePath)
			if len(tc.wantErr) > 0 {
				require.ErrorContains(t, err, tc.wantErr, "reader.ReadFile should have failed")
				return
			}
			require.NoError(t, err, "reader.ReadFile should not have failed.")
		})
	}
}

// TestOpenRootOS exercises openRootOS and basic operations (Lstat, Open) against real directories
// and files on disk.
// This is useful because it validates that openRootOS opens real directory descriptors,
// fails early on invalid root targets (missing paths or regular files), and correctly reads
// file contents and directory metadata across nested paths.
func TestOpenRootOS(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		missingDir  bool
		fileAsRoot  bool
		missingPath bool
		nested      bool
		isDir       bool

		wantErr     bool
		wantContent string
	}{
		"Fails on non-existent directory":    {missingDir: true, wantErr: true},
		"Fails on file instead of directory": {fileAsRoot: true, wantErr: true},
		"Reads regular file":                 {wantContent: "hello real fs"},
		"Reads nested file in subdirectory":  {nested: true, wantContent: "world real fs"},
		"Stats directory":                    {isDir: true},
		"Fails on missing path":              {missingPath: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello real fs"), 0o600), "Setup: failed to write test file")
			subDir := filepath.Join(dir, "sub")
			require.NoError(t, os.MkdirAll(subDir, 0o700), "Setup: failed to create subdirectory")
			require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("world real fs"), 0o600), "Setup: failed to write nested test file")

			rootDir := dir
			if tc.missingDir {
				rootDir = filepath.Join(t.TempDir(), "does-not-exist")
			}
			if tc.fileAsRoot {
				rootDir = filepath.Join(dir, "hello.txt")
			}

			root, err := daemon.OpenRoot(rootDir)
			if tc.missingDir || tc.fileAsRoot {
				require.Error(t, err, "OpenRoot should have failed on invalid root target")
				return
			}
			require.NoError(t, err, "Setup: could not open root")
			t.Cleanup(func() { _ = root.Close() })

			target := "hello.txt"
			if tc.nested {
				target = "sub/nested.txt"
			}
			if tc.isDir {
				target = "sub"
			}
			if tc.missingPath {
				target = "missing.txt"
			}

			fi, err := root.Lstat(target)
			if tc.wantErr {
				require.Error(t, err, "root.Lstat should have failed for missing path")
				_, err = root.Open(target)
				require.Error(t, err, "root.Open should have failed for missing path")
				return
			}

			require.NoError(t, err, "root.Lstat should not have failed")
			if tc.isDir {
				require.True(t, fi.IsDir(), "root.Lstat should report directory")
				return
			}

			require.False(t, fi.IsDir(), "root.Lstat should not report regular file as directory")
			rc, err := root.Open(target)
			require.NoError(t, err, "root.Open should not have failed")
			data, err := io.ReadAll(rc)
			require.NoError(t, err, "reading opened file contents should succeed")
			require.NoError(t, rc.Close(), "closing opened file should succeed")
			require.Equal(t, tc.wantContent, string(data), "opened file contents should match expected data")
		})
	}
}

// TestOpenRootOS_ConfinesPathResolution asserts kernel-level path resolution confinement
// using openat2 flags (RESOLVE_NO_SYMLINKS and RESOLVE_BENEATH) against the real filesystem.
// This is useful because it guarantees the descriptor boundary cannot be escaped via absolute
// paths, ".." parent traversals, or symlinks pointing outside the designated root directory,
// preventing TOCTOU directory traversal attacks.
func TestOpenRootOS_ConfinesPathResolution(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		target         string
		fileData       string
		symlinkTarget  string
		outsideSymlink bool
		escapesRoot    bool

		wantLstatErr  bool
		wantLstatLink bool
		wantOpenErr   bool
		wantOpenData  string
	}{
		"Open inside the root succeeds": {
			target:       "ok.txt",
			fileData:     "ok",
			wantOpenData: "ok",
		},
		"Lstat and Open refuse absolute paths": {
			target:       "/etc/passwd",
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat and Open refuse .. that escapes the root": {
			escapesRoot:  true,
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat and Open refuse in-root symlinks pointing outside the root": {
			target:         "leak.txt",
			outsideSymlink: true,
			wantLstatLink:  true,
			wantOpenErr:    true,
		},
		"Open refuses symlinks with absolute targets": {
			target:        "abs.txt",
			symlinkTarget: "/etc/passwd",
			wantOpenErr:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			path := tc.target

			switch {
			case tc.escapesRoot:
				outside := t.TempDir()
				secret := filepath.Join(outside, "secret.txt")
				require.NoError(t, os.WriteFile(secret, []byte("outside"), 0o600), "Setup: failed to write outside secret file")
				rel, err := filepath.Rel(rootDir, secret)
				require.NoError(t, err, "Setup: failed to compute relative path")
				require.True(t, strings.HasPrefix(rel, ".."), "Setup: outside file must lie outside the root")
				path = rel
			case tc.outsideSymlink:
				outside := t.TempDir()
				secret := filepath.Join(outside, "secret.txt")
				require.NoError(t, os.WriteFile(secret, []byte("outside"), 0o600), "Setup: failed to write outside secret file")
				require.NoError(t, os.Symlink(secret, filepath.Join(rootDir, tc.target)), "Setup: failed to create symlink")
			case tc.symlinkTarget != "":
				require.NoError(t, os.Symlink(tc.symlinkTarget, filepath.Join(rootDir, tc.target)), "Setup: failed to create symlink to "+tc.symlinkTarget)
			case tc.fileData != "":
				require.NoError(t, os.WriteFile(filepath.Join(rootDir, tc.target), []byte(tc.fileData), 0o600), "Setup: failed to write test file")
			}

			root, err := daemon.OpenRoot(rootDir)
			require.NoError(t, err, "Setup: could not open root")
			t.Cleanup(func() { _ = root.Close() })

			switch {
			case tc.wantLstatErr:
				_, err = root.Lstat(path)
				require.Error(t, err, "root.Lstat should fail for path escaping root")
			case tc.wantLstatLink:
				fi, err := root.Lstat(path)
				require.NoError(t, err, "root.Lstat should not fail for symlink within root")
				require.NotZero(t, fi.Mode()&fs.ModeSymlink,
					"Lstat must report the symlink itself, not its target")
			default:
				_, err = root.Lstat(path)
				require.NoError(t, err, "root.Lstat should not fail for path within root")
			}

			switch {
			case tc.wantOpenErr:
				_, err = root.Open(path)
				require.Error(t, err, "root.Open should fail for path escaping root")
			case tc.wantOpenData != "":
				f, err := root.Open(path)
				require.NoError(t, err, "root.Open should succeed for path within root")
				t.Cleanup(func() { _ = f.Close() })

				data, err := io.ReadAll(f)
				require.NoError(t, err, "reading opened file contents should succeed")
				require.Equal(t, tc.wantOpenData, string(data), "read contents should match expected")
			default:
				_, err = root.Open(path)
				require.NoError(t, err, "root.Open should succeed")
			}
		})
	}
}

// TestOpenRootOS_LifecycleAndErrors asserts descriptor lifecycle invariants and error handling
// for openat2Root instances.
// This is useful because it verifies that closing a root descriptor is idempotent (double close
// is a safe no-op), subsequent operations on closed descriptors fail predictably with expected
// errors, and operations on invalid file descriptors fail closed rather than panicking or misbehaving.
func TestOpenRootOS_LifecycleAndErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		closeTwice bool
		closed     bool
		invalidFd  bool
		openOp     bool
		target     string
		wantErr    string
	}{
		"Double close is a no-op": {
			closeTwice: true,
		},
		"Lstat on closed root returns error": {
			closed:  true,
			target:  "file.txt",
			wantErr: "root is closed",
		},
		"Open on closed root returns error": {
			closed:  true,
			openOp:  true,
			target:  "file.txt",
			wantErr: "root is closed",
		},
		"Lstat on root itself with invalid descriptor returns error": {
			invalidFd: true,
			target:    ".",
		},
		"Lstat on missing intermediate parent directory fails": {
			target: "missing_parent/file.txt",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			root, err := daemon.OpenRoot(rootDir)
			require.NoError(t, err, "Setup: could not open root")
			t.Cleanup(func() { _ = root.Close() })

			if tc.closeTwice {
				require.NoError(t, root.Close(), "first root.Close should succeed")
				require.NoError(t, root.Close(), "second root.Close should succeed as a no-op")
				return
			}
			if tc.closed {
				require.NoError(t, root.Close(), "Setup: failed to close root")
			}
			if tc.invalidFd {
				root = daemon.NewOpenat2RootForTest(99999, "bad")
			}

			if tc.openOp {
				_, err := root.Open(tc.target)
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr, "Open on closed root should return root is closed error")
				} else {
					require.Error(t, err, "root.Open should fail")
				}
				return
			}

			_, err = root.Lstat(tc.target)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr, "Lstat on closed root should return root is closed error")
			} else {
				require.Error(t, err, "root.Lstat should fail")
			}
		})
	}
}

// TestOpenRootOS_RealSpecialFiles asserts that Lstat on openat2Root correctly detects non-regular
// special files created on a real filesystem (such as FIFOs and UNIX domain sockets).
// This is useful because secure file readers must detect and reject irregular file types
// that could otherwise block indefinitely on read or trigger unexpected IPC side effects.
func TestOpenRootOS_RealSpecialFiles(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		isFIFO   bool
		isSocket bool
		wantMode fs.FileMode
	}{
		"FIFO named pipe": {
			isFIFO:   true,
			wantMode: fs.ModeNamedPipe,
		},
		"Unix socket": {
			isSocket: true,
			wantMode: fs.ModeSocket,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			var name string

			switch {
			case tc.isFIFO:
				name = "test.fifo"
				err := syscall.Mkfifo(filepath.Join(rootDir, name), 0o600)
				require.NoError(t, err, "Setup: could not create FIFO")
			case tc.isSocket:
				name = "test.sock"
				l, err := net.Listen("unix", filepath.Join(rootDir, name))
				require.NoError(t, err, "Setup: could not create unix socket")
				t.Cleanup(func() { _ = l.Close() })
			}

			root, err := daemon.OpenRoot(rootDir)
			require.NoError(t, err, "Setup: could not open root")
			t.Cleanup(func() { _ = root.Close() })

			fi, err := root.Lstat(name)
			require.NoError(t, err, "Lstat should succeed on special file")
			require.Equal(t, name, fi.Name(), "file name should match")
			require.NotZero(t, fi.Mode()&tc.wantMode, "file mode should include expected mode bits")
		})
	}
}

// TestFileInfoFromStat asserts that fileInfoFromStat accurately converts raw unix.Stat_t
// kernel metadata into os.FileInfo representations across all standard POSIX file types
// (regular files, directories, symlinks, FIFOs, sockets, character devices, and block devices).
// This is useful because the openat2-based reader synthesizes FileInfo directly from statx/fstatat
// system calls and relies on exact mode bit masks, timestamps, and size conversions.
func TestFileInfoFromStat(t *testing.T) {
	t.Parallel()

	now := time.Now()
	sec := now.Unix()
	nsec := int64(now.Nanosecond())

	testCases := map[string]struct {
		stat     unix.Stat_t
		name     string
		wantDir  bool
		wantMode fs.FileMode
	}{
		"Regular file": {
			stat: unix.Stat_t{
				Mode: unix.S_IFREG | 0o644,
				Size: 42,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "regular.txt",
			wantDir:  false,
			wantMode: 0o644,
		},
		"Directory": {
			stat: unix.Stat_t{
				Mode: unix.S_IFDIR | 0o755,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "dir",
			wantDir:  true,
			wantMode: fs.ModeDir | 0o755,
		},
		"Symlink": {
			stat: unix.Stat_t{
				Mode: unix.S_IFLNK | 0o777,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "symlink",
			wantDir:  false,
			wantMode: fs.ModeSymlink | 0o777,
		},
		"Named pipe FIFO": {
			stat: unix.Stat_t{
				Mode: unix.S_IFIFO | 0o600,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "fifo",
			wantDir:  false,
			wantMode: fs.ModeNamedPipe | 0o600,
		},
		"Socket": {
			stat: unix.Stat_t{
				Mode: unix.S_IFSOCK | 0o600,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "sock",
			wantDir:  false,
			wantMode: fs.ModeSocket | 0o600,
		},
		"Character device": {
			stat: unix.Stat_t{
				Mode: unix.S_IFCHR | 0o660,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "null",
			wantDir:  false,
			wantMode: fs.ModeDevice | fs.ModeCharDevice | 0o660,
		},
		"Block device": {
			stat: unix.Stat_t{
				Mode: unix.S_IFBLK | 0o660,
				Mtim: unix.Timespec{Sec: sec, Nsec: nsec},
			},
			name:     "sda",
			wantDir:  false,
			wantMode: fs.ModeDevice | 0o660,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fi := daemon.FileInfoFromStat(tc.name, &tc.stat)
			require.Equal(t, tc.name, fi.Name(), "file name should match")
			require.Equal(t, tc.wantDir, fi.IsDir(), "isDir should match expected")
			require.Equal(t, tc.wantMode, fi.Mode(), "mode should match expected")
			require.Equal(t, tc.stat.Size, fi.Size(), "size should match expected")
			require.Equal(t, time.Unix(sec, nsec), fi.ModTime(), "modTime should match expected")
			require.NotNil(t, fi.Sys(), "Sys should not be nil")
		})
	}
}

type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     any
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return m.sys }

// mockRootFs implements daemon.RootFs for tests. Paths seen by Lstat and Open
// are relative to the root.
type mockRootFs struct {
	infos    map[string]fs.FileInfo // per relative path metadata
	contents map[string]string      // per relative path file contents

	lstatErr error // error returned by Lstat regardless of path
	openErr  error // error returned by Open regardless of path
	closed   bool
}

func (m *mockRootFs) Lstat(name string) (fs.FileInfo, error) {
	if m.lstatErr != nil {
		return nil, m.lstatErr
	}
	// Lstat(".") is the call the reader uses to validate the root itself; the mock
	// returns a default root FileInfo when "." is not explicitly configured, matching
	// what a real os.Root.Lstat(".") reports on a directory with mode 0700 owned by
	// the test user. (Test cases that need to exercise a non-conforming root move the
	// validation through TestDefaultSecureReader_RealFS, which runs against the real
	// filesystem and cannot bypass ownership.)
	if filepath.Clean(name) == "." {
		return secureRootInfo, nil
	}
	fi, ok := m.infos[filepath.Clean(name)]
	if !ok {
		return nil, errors.New("no metadata configured for " + name)
	}
	return fi, nil
}

func (m *mockRootFs) Open(name string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	content, ok := m.contents[filepath.Clean(name)]
	if !ok {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return io.NopCloser(bytes.NewReader([]byte(content))), nil
}

func (m *mockRootFs) Close() error {
	m.closed = true
	return nil
}

var secureRootInfo = mockFileInfo{name: "public", isDir: true, mode: fs.ModeDir | 0700, sys: &syscall.Stat_t{Uid: 0, Gid: 0}}

func secureDirInfo(name string) fs.FileInfo {
	return mockFileInfo{name: name, isDir: true, mode: fs.ModeDir | 0700, sys: &syscall.Stat_t{Uid: 0, Gid: 0}}
}

func secureFileInfo(name string) fs.FileInfo {
	return mockFileInfo{name: name, mode: 0600, sys: &syscall.Stat_t{Uid: 0, Gid: 0}}
}
