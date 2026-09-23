package daemon_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

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

	expectedUID, expectedGID := daemon.ExpectedOwnerForTest()

	testCases := map[string]struct {
		stat    daemon.FileStat
		path    string
		wantErr string
	}{
		"Valid directory": {
			stat: secureDirInfo("dir"),
			path: "/dir",
		},
		"Valid regular file": {
			stat: secureFileInfo("file"),
			path: "/file",
		},
		"Invalid UID on file": {
			stat:    daemon.FileStat{Mode: unix.S_IFREG | 0o600, UID: 99999, GID: expectedGID},
			path:    "/file",
			wantErr: fmt.Sprintf(`refused "/file": not strictly owned by root (uid 99999, gid %d)`, expectedGID),
		},
		"Invalid GID on file": {
			stat:    daemon.FileStat{Mode: unix.S_IFREG | 0o600, UID: expectedUID, GID: 99999},
			path:    "/file",
			wantErr: fmt.Sprintf(`refused "/file": not strictly owned by root (uid %d, gid 99999)`, expectedUID),
		},
		"Invalid directory mode": {
			stat:    daemon.FileStat{Mode: unix.S_IFDIR | 0o755, UID: expectedUID, GID: expectedGID},
			path:    "/dir",
			wantErr: `refused directory "/dir": not strictly owned by root (mode 0755)`,
		},
		"Invalid file mode": {
			stat:    daemon.FileStat{Mode: unix.S_IFREG | 0o644, UID: expectedUID, GID: expectedGID},
			path:    "/file",
			wantErr: `refused file "/file": not strictly owned by root (mode 0644)`,
		},
		"Refuses file with setuid bit": {
			stat:    daemon.FileStat{Mode: unix.S_IFREG | unix.S_ISUID | 0o600, UID: expectedUID, GID: expectedGID},
			path:    "/file",
			wantErr: `refused "/file": special permission bits (setuid/setgid/sticky) are not permitted`,
		},
		"Refuses file with setgid bit": {
			stat:    daemon.FileStat{Mode: unix.S_IFREG | unix.S_ISGID | 0o600, UID: expectedUID, GID: expectedGID},
			path:    "/file",
			wantErr: `refused "/file": special permission bits (setuid/setgid/sticky) are not permitted`,
		},
		"Refuses directory with sticky bit": {
			stat:    daemon.FileStat{Mode: unix.S_IFDIR | unix.S_ISVTX | 0o700, UID: expectedUID, GID: expectedGID},
			path:    "/dir",
			wantErr: `refused "/dir": special permission bits (setuid/setgid/sticky) are not permitted`,
		},
		"Refuses symlink": {
			stat:    daemon.FileStat{Mode: unix.S_IFLNK | 0o777, UID: expectedUID, GID: expectedGID},
			path:    "/link",
			wantErr: `refused "/link": symlinks are not permitted`,
		},
		"Refuses FIFO": {
			stat:    daemon.FileStat{Mode: unix.S_IFIFO | 0o600, UID: expectedUID, GID: expectedGID},
			path:    "/fifo",
			wantErr: fmt.Sprintf(`refused "/fifo": irregular file type (mode 0%o)`, unix.S_IFIFO|0o600),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := daemon.DefaultValidate(tc.path, tc.stat)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr, "DefaultValidate should return expected error")
			} else {
				require.NoError(t, err, "DefaultValidate should succeed on valid path/info")
			}
		})
	}
}

// TestDefaultSecureReader drives the mocked-seam cases: validation logic, walk-loop error
// handling, and lifecycle (root is always closed). Paths that depend on real filesystem behavior (a symlink
// rootDir, a missing rootDir) live in TestDefaultSecureReader_RealFS below; cases that need
// root-owned 0600/0700 paths live in TestDefaultSecureReader_RealFS_RefusesNonRootOwnership.
func TestDefaultSecureReader(t *testing.T) {
	t.Parallel()

	expectedUID, expectedGID := daemon.ExpectedOwnerForTest()

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
				infos: map[string]daemon.FileStat{
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
		"Rejected when the root directory has insecure permissions": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					".": {Name: ".", Mode: unix.S_IFDIR | 0o755, UID: expectedUID, GID: expectedGID},
				},
			},
			targetPath: "file.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when stating a component fails": {
			root:       &mockRootFs{lstatErr: errors.New("disk failure")},
			targetPath: "sub/file.txt",
			wantErr:    "could not stat",
			wantClosed: true,
		},
		"Rejected on symlink file": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"symlink.txt": {Name: "symlink.txt", Mode: unix.S_IFLNK | 0o777, UID: expectedUID, GID: expectedGID},
				},
			},
			targetPath: "symlink.txt",
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on intermediate symlink directory": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"symlink_dir": {Name: "symlink_dir", Mode: unix.S_IFLNK | 0o777, UID: expectedUID, GID: expectedGID},
				},
			},
			targetPath: "symlink_dir/file.txt",
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on irregular file type": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"pipe": {Name: "pipe", Mode: unix.S_IFIFO | 0o600, UID: expectedUID, GID: expectedGID},
				},
			},
			targetPath: "pipe",
			wantErr:    "irregular file type",
			wantClosed: true,
		},
		"Rejected when an intermediate directory has insecure permissions": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"sub":          {Name: "sub", Mode: unix.S_IFDIR | 0o755, UID: expectedUID, GID: expectedGID},
					"sub/file.txt": secureFileInfo("file.txt"),
				},
			},
			targetPath: "sub/file.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when the file has insecure permissions": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"file.txt": {Name: "file.txt", Mode: unix.S_IFREG | 0o644, UID: expectedUID, GID: expectedGID},
				},
			},
			targetPath: "file.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when opening the target fails": {
			root: &mockRootFs{
				infos:   map[string]daemon.FileStat{"file.txt": secureFileInfo("file.txt")},
				openErr: errors.New("open error: permission denied"),
			},
			targetPath: "file.txt",
			wantErr:    "could not read",
			wantClosed: true,
		},
		"Rejected when target descriptor was swapped with illegitimate inode before validation": {
			root: &mockRootFs{
				infos: map[string]daemon.FileStat{
					"secret.txt": secureFileInfo("secret.txt"),
				},
				openFiles: map[string]daemon.FileStat{
					"secret.txt": {Name: "secret.txt", Mode: unix.S_IFREG | 0o666, UID: expectedUID, GID: expectedGID},
				},
				contents: map[string]string{"secret.txt": "compromised data"},
			},
			targetPath: "secret.txt",
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when stating the open target descriptor fails": {
			root: &mockRootFs{
				infos:       map[string]daemon.FileStat{"file.txt": secureFileInfo("file.txt")},
				fileStatErr: errors.New("descriptor bad"),
			},
			targetPath: "file.txt",
			wantErr:    "could not stat",
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
		wantErr string
	}{
		"Refuses a symlink rootDir": {symlink: true, wantErr: "root is a symlink"},
		"Fails on missing rootDir":  {missing: true, wantErr: "could not stat"},
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
// This is useful because openRootOS encapsulates real OS-level directory opening and openat2-based
// confined traversal.
func TestOpenRootOS(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		missingDir       bool
		missingParent    bool
		fileAsRoot       bool
		mountPointAsRoot bool
		filesystemRoot   bool
		missingPath      bool
		nested           bool
		isDir            bool

		wantErr     bool
		wantContent string
	}{
		"Fails on non-existent directory":        {missingDir: true, wantErr: true},
		"Fails on non-existent parent directory": {missingParent: true, wantErr: true},
		"Fails on file instead of directory":     {fileAsRoot: true, wantErr: true},
		"Fails on mount point directory":         {mountPointAsRoot: true, wantErr: true},
		"Opens filesystem root":                  {filesystemRoot: true, isDir: true},
		"Reads regular file":                     {wantContent: "hello real fs"},
		"Reads nested file in subdirectory":      {nested: true, wantContent: "world real fs"},
		"Stats directory":                        {isDir: true},
		"Fails on missing path":                  {missingPath: true, wantErr: true},
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
			if tc.missingParent {
				rootDir = "/nonexistent-parent/does-not-exist"
			}
			if tc.fileAsRoot {
				rootDir = filepath.Join(dir, "hello.txt")
			}
			if tc.mountPointAsRoot {
				rootDir = "/proc"
			}
			if tc.filesystemRoot {
				rootDir = "/"
			}

			root, err := daemon.OpenRoot(rootDir)
			if tc.missingDir || tc.missingParent || tc.fileAsRoot || tc.mountPointAsRoot {
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
			if tc.filesystemRoot {
				target = "etc"
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
			st, err := rc.Stat()
			require.NoError(t, err, "rc.Stat should succeed on open file")
			require.False(t, st.IsDir(), "rc.Stat should report regular file")
			data, err := io.ReadAll(rc)
			require.NoError(t, err, "reading opened file contents should succeed")
			require.NoError(t, rc.Close(), "closing opened file should succeed")
			require.Equal(t, tc.wantContent, string(data), "opened file contents should match expected data")
		})
	}
}

// TestOpenRootOS_ConfinesPathResolution asserts kernel-level path resolution confinement
// using openat2 flags (RESOLVE_NO_SYMLINKS and RESOLVE_BENEATH).
// This is useful because it validates that attempts to escape the root directory via symlinks,
// absolute paths, or ".." path traversal sequences are rejected at the syscall layer.
func TestOpenRootOS_ConfinesPathResolution(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "valid.txt"), []byte("valid content"), 0o600), "Setup: failed to write valid test file")

	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "outside.txt"), []byte("outside content"), 0o600), "Setup: failed to write outside test file")

	// Symlink pointing outside the root
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, "outside.txt"), filepath.Join(rootDir, "escape_link.txt")), "Setup: failed to create escaping symlink")

	root, err := daemon.OpenRoot(rootDir)
	require.NoError(t, err, "Setup: could not open root")
	t.Cleanup(func() { _ = root.Close() })

	testCases := map[string]struct {
		path          string
		wantLstatErr  bool
		wantLstatLink bool
		wantOpenErr   bool
	}{
		"Valid file within root succeeds": {
			path: "valid.txt",
		},
		"Lstat and Open refuse absolute paths": {
			path:         filepath.Join(rootDir, "valid.txt"),
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat and Open refuse .. that escapes the root": {
			path:         "../outside.txt",
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat and Open refuse in-root symlinks pointing outside the root": {
			path:          "escape_link.txt",
			wantLstatLink: true,
			wantOpenErr:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.path
			if strings.HasPrefix(name, "Lstat and Open refuse absolute paths") {
				// Normalize absolute path for testing
				path = filepath.Clean(path)
			}

			switch {
			case tc.wantLstatErr:
				_, err = root.Lstat(path)
				require.Error(t, err, "root.Lstat should fail for path escaping root")
			case tc.wantLstatLink:
				fi, err := root.Lstat(path)
				require.NoError(t, err, "root.Lstat should not fail for symlink within root")
				require.True(t, fi.IsSymlink(), "Lstat must report the symlink itself, not its target")
			default:
				_, err = root.Lstat(path)
				require.NoError(t, err, "root.Lstat should not fail for path within root")
			}

			if tc.wantOpenErr {
				_, err = root.Open(path)
				require.Error(t, err, "root.Open should fail for path escaping root or traversing symlink")
			} else {
				rc, err := root.Open(path)
				require.NoError(t, err, "root.Open should succeed for valid path")
				require.NoError(t, rc.Close(), "Close should succeed")
			}
		})
	}
}

// TestOpenRootOS_CloserLifecycle asserts that Open and Lstat on an already-closed root
// consistently return an error, preventing use-after-close bugs.
// This is useful because leaking or reusing file descriptors after Close can lead to descriptor
// confusion and unpredictable behavior.
func TestOpenRootOS_CloserLifecycle(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		target        string
		closedRoot    bool
		invalidRootFd bool
		missingParent bool
		wantErr       string
	}{
		"Open on closed root returns error": {
			target:     "file.txt",
			closedRoot: true,
			wantErr:    "root is closed",
		},
		"Lstat on closed root returns error": {
			target:     "file.txt",
			closedRoot: true,
			wantErr:    "root is closed",
		},
		"Lstat on root itself when closed returns error": {
			target:     ".",
			closedRoot: true,
			wantErr:    "root is closed",
		},
		"Lstat on root itself with invalid descriptor returns error": {
			target:        ".",
			invalidRootFd: true,
		},
		"Lstat on missing intermediate parent directory fails": {
			target:        "nonexistent/file.txt",
			missingParent: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			root, err := daemon.OpenRoot(dir)
			require.NoError(t, err, "Setup: could not open root")

			if tc.closedRoot {
				require.NoError(t, root.Close(), "Close should succeed")
			} else if tc.invalidRootFd {
				root = daemon.NewOpenat2RootForTest(-999, dir)
			} else {
				t.Cleanup(func() { _ = root.Close() })
			}

			if strings.HasPrefix(name, "Open") {
				_, err = root.Open(tc.target)
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
		wantMode uint32
	}{
		"FIFO named pipe": {
			isFIFO:   true,
			wantMode: unix.S_IFIFO,
		},
		"Unix socket": {
			isSocket: true,
			wantMode: unix.S_IFSOCK,
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
			require.Equal(t, name, fi.Name, "file name should match")
			require.NotZero(t, fi.Mode&tc.wantMode, "file mode should include expected mode bits")
		})
	}
}

// mockRootFs implements daemon.RootFs for tests. Paths seen by Lstat and Open
// are relative to the root.
type mockConfinedFile struct {
	io.ReadCloser
	stat    daemon.FileStat
	statErr error
}

func (m *mockConfinedFile) Stat() (daemon.FileStat, error) {
	if m.statErr != nil {
		return daemon.FileStat{}, m.statErr
	}
	return m.stat, nil
}

type mockRootFs struct {
	infos    map[string]daemon.FileStat // per relative path metadata
	contents map[string]string          // per relative path file contents

	lstatErr    error                      // error returned by Lstat regardless of path
	openErr     error                      // error returned by Open regardless of path
	openFiles   map[string]daemon.FileStat // override descriptor stat to simulate inode swap
	fileStatErr error                      // error returned by file Stat()
	closed      bool
}

func (m *mockRootFs) Lstat(name string) (daemon.FileStat, error) {
	if m.lstatErr != nil {
		return daemon.FileStat{}, m.lstatErr
	}
	// Lstat(".") is the call the reader uses to validate the root itself; the mock
	// returns a default root FileStat when "." is not explicitly configured, matching
	// what a real os.Root.Lstat(".") reports on a directory with mode 0700 owned by
	// the test user.
	if filepath.Clean(name) == "." {
		if fi, ok := m.infos["."]; ok {
			return fi, nil
		}
		return secureRootInfo(), nil
	}
	fi, ok := m.infos[filepath.Clean(name)]
	if !ok {
		return daemon.FileStat{}, errors.New("no metadata configured for " + name)
	}
	return fi, nil
}

func (m *mockRootFs) Open(name string) (daemon.ConfinedFile, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	clean := filepath.Clean(name)
	st, ok := m.openFiles[clean]
	if !ok {
		st = m.infos[clean]
	}
	content, hasContent := m.contents[clean]
	var rc io.ReadCloser
	if !hasContent {
		rc = io.NopCloser(bytes.NewReader(nil))
	} else {
		rc = io.NopCloser(bytes.NewReader([]byte(content)))
	}
	return &mockConfinedFile{
		ReadCloser: rc,
		stat:       st,
		statErr:    m.fileStatErr,
	}, nil
}

func (m *mockRootFs) Close() error {
	m.closed = true
	return nil
}

func secureRootInfo() daemon.FileStat {
	u, g := daemon.ExpectedOwnerForTest()
	return daemon.FileStat{Name: "public", Mode: unix.S_IFDIR | 0o700, UID: u, GID: g}
}

func secureDirInfo(name string) daemon.FileStat {
	u, g := daemon.ExpectedOwnerForTest()
	return daemon.FileStat{Name: name, Mode: unix.S_IFDIR | 0o700, UID: u, GID: g}
}

func secureFileInfo(name string) daemon.FileStat {
	u, g := daemon.ExpectedOwnerForTest()
	return daemon.FileStat{Name: name, Mode: unix.S_IFREG | 0o600, UID: u, GID: g}
}
