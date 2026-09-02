//go:build !windows

package daemon_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/stretchr/testify/require"
)

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

// mockOSFs implements daemon.OsFs for tests. Paths seen by Lstat are absolute;
// OpenRoot returns the configured mockRootFs.
type mockOSFs struct {
	rootInfo fs.FileInfo // metadata reported for the root directory
	rootErr  error       // error returned when stating the root directory

	openRootErr error
	root        *mockRootFs
}

func (m mockOSFs) Lstat(path string) (fs.FileInfo, error) {
	if m.rootErr != nil {
		return nil, m.rootErr
	}
	if m.rootInfo == nil {
		return nil, fmt.Errorf("no metadata configured for %q", path)
	}
	return m.rootInfo, nil
}

func (m mockOSFs) OpenRoot(path string) (daemon.RootFs, error) {
	if m.openRootErr != nil {
		return nil, m.openRootErr
	}
	if m.root == nil {
		return nil, errors.New("no root configured")
	}
	return m.root, nil
}

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
	fi, ok := m.infos[filepath.Clean(name)]
	if !ok {
		return nil, fmt.Errorf("no metadata configured for %q", name)
	}
	return fi, nil
}

func (m *mockRootFs) Open(name string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	content, ok := m.contents[filepath.Clean(name)]
	if !ok {
		return nil, fmt.Errorf("no contents configured for %q", name)
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
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultSecureReader(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockFS      mockOSFs
		rootDir     string
		targetPath  string
		wantContent string
		wantErr     string
		wantClosed  bool
	}{
		"Valid nested file is read and the root is closed": {
			rootDir:    "/root/public",
			targetPath: "sub/secret.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"sub":            secureDirInfo("sub"),
						"sub/secret.txt": secureFileInfo("secret.txt"),
					},
					contents: map[string]string{"sub/secret.txt": "hello mock secret"}, //nolint:gosec // test fixture, not a credential
				},
			},
			wantContent: "hello mock secret",
			wantClosed:  true,
		},
		"Rejected when the root directory is not root-owned": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS: mockOSFs{
				rootInfo: mockFileInfo{name: "public", isDir: true, mode: fs.ModeDir | 0700, sys: &syscall.Stat_t{Uid: 1000, Gid: 1000}},
			},
			wantErr: "not strictly owned by root",
		},
		"Rejected when the root directory is a symlink": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS: mockOSFs{
				rootInfo: mockFileInfo{name: "public", mode: fs.ModeSymlink | 0700, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
			},
			wantErr: "symlinks are not permitted",
		},
		"Rejected when stating the root fails": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS:     mockOSFs{rootErr: errors.New("disk failure")},
			wantErr:    "could not stat",
		},
		"Rejected when opening the root fails": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS: mockOSFs{
				rootInfo:    secureRootInfo,
				openRootErr: errors.New("permission denied"),
			},
			wantErr: "could not open root",
		},
		"Rejected when the target path escapes the root": {
			rootDir:    "/root/public",
			targetPath: "../outside.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root:     &mockRootFs{},
			},
			wantErr:    "path escapes root",
			wantClosed: true,
		},
		"Rejected when the target path is absolute": {
			rootDir:    "/root/public",
			targetPath: "/etc/passwd",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root:     &mockRootFs{},
			},
			wantErr:    "path escapes root",
			wantClosed: true,
		},
		"Rejected when stating a component fails": {
			rootDir:    "/root/public",
			targetPath: "sub/file.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root:     &mockRootFs{lstatErr: errors.New("disk failure")},
			},
			wantErr:    "could not stat",
			wantClosed: true,
		},
		"Rejected on symlink file": {
			rootDir:    "/root/public",
			targetPath: "symlink.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"symlink.txt": mockFileInfo{name: "symlink.txt", mode: fs.ModeSymlink | 0777, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
					},
				},
			},
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on intermediate symlink directory": {
			rootDir:    "/root/public",
			targetPath: "symlink_dir/file.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"symlink_dir": mockFileInfo{name: "symlink_dir", mode: fs.ModeSymlink | 0777, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
					},
				},
			},
			wantErr:    "symlinks are not permitted",
			wantClosed: true,
		},
		"Rejected on irregular file type": {
			rootDir:    "/root/public",
			targetPath: "pipe",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"pipe": mockFileInfo{name: "pipe", mode: fs.ModeNamedPipe | 0600, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
					},
				},
			},
			wantErr:    "irregular file type",
			wantClosed: true,
		},
		"Rejected when an intermediate directory has insecure permissions": {
			rootDir:    "/root/public",
			targetPath: "sub/file.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"sub":          mockFileInfo{name: "sub", isDir: true, mode: fs.ModeDir | 0755, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
						"sub/file.txt": secureFileInfo("file.txt"),
					},
				},
			},
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when the file has insecure permissions": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos: map[string]fs.FileInfo{
						"file.txt": mockFileInfo{name: "file.txt", mode: 0644, sys: &syscall.Stat_t{Uid: 0, Gid: 0}},
					},
				},
			},
			wantErr:    "not strictly owned by root",
			wantClosed: true,
		},
		"Rejected when opening the target fails": {
			rootDir:    "/root/public",
			targetPath: "file.txt",
			mockFS: mockOSFs{
				rootInfo: secureRootInfo,
				root: &mockRootFs{
					infos:   map[string]fs.FileInfo{"file.txt": secureFileInfo("file.txt")},
					openErr: errors.New("open error: permission denied"),
				},
			},
			wantErr:    "could not read",
			wantClosed: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reader := daemon.NewDefaultSecureReader(tc.mockFS)
			got, err := reader.ReadFile(tc.rootDir, tc.targetPath)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantContent, string(got))
			}

			if tc.wantClosed {
				require.True(t, tc.mockFS.root.closed, "the root must be closed after ReadFile")
			}
		})
	}
}

func TestDefaultSecureReader_RealFS_RefusesNonRootOwnership(t *testing.T) {
	t.Parallel()

	// The real filesystem backend is exercised here. Because the test runs
	// unprivileged, every file it creates is owned by the current user (uid != 0),
	// so validation must refuse it with "not strictly owned by root".
	// This gives us coverage of the wiring from defaultSecureReader through
	// realOSFs / os.Root into ownershipOf without needing root privileges.
	reader := daemon.NewDefaultSecureReader(daemon.RealOSFs{})

	rootDir := t.TempDir()

	// Make the root directory's mode correct (0700) so the refusal is
	// attributable to ownership, not to permissions.
	require.NoError(t, os.Chmod(rootDir, 0o700)) //nolint:gosec // test exercises 0700 directory validation

	// A regular file directly under the root.
	filePath := filepath.Join(rootDir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o600))

	_, err := reader.ReadFile(rootDir, "hello.txt")
	require.ErrorContains(t, err, "not strictly owned by root",
		"real FS should refuse a non-root-owned root directory (and its contents)")

	// A nested file: the intermediate directory is also correctly permissioned,
	// but still non-root-owned, so it must be refused as well.
	subDir := filepath.Join(rootDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	nestedPath := filepath.Join(subDir, "nested.txt")
	require.NoError(t, os.WriteFile(nestedPath, []byte("world"), 0o600))

	_, err = reader.ReadFile(rootDir, "sub/nested.txt")
	require.ErrorContains(t, err, "not strictly owned by root")
}

func TestRealOSFs_OpenRoot_AndAdapter(t *testing.T) {
	t.Parallel()

	// Exercise the production filesystem adapter directly, bypassing the
	// ownership checks. This is the only way to cover realOSFs.OpenRoot and
	// osRootAdapter.{Lstat,Open,Close} without root privileges — the
	// ownership test above already covers the validation path through
	// realOSFs.Lstat.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello real fs"), 0o600))
	subDir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	nestedPath := filepath.Join(subDir, "nested.txt")
	require.NoError(t, os.WriteFile(nestedPath, []byte("world real fs"), 0o600))

	fsys := daemon.RealOSFs{}

	// realOSFs.Lstat on an existing path succeeds
	fi, err := fsys.Lstat(dir)
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	// realOSFs.Lstat on a missing path fails
	_, err = fsys.Lstat(filepath.Join(dir, "does-not-exist"))
	require.Error(t, err)

	// realOSFs.OpenRoot on a directory succeeds and returns a usable Root
	root, err := fsys.OpenRoot(dir)
	require.NoError(t, err)
	require.NotNil(t, root)
	defer func() {
		// Close should succeed once; second close should error (os.Root is single-use)
		require.NoError(t, root.Close())
	}()

	// adapter Lstat for existing file and directory
	fi, err = root.Lstat("hello.txt")
	require.NoError(t, err)
	require.False(t, fi.IsDir())
	require.Equal(t, "hello.txt", fi.Name())

	fi, err = root.Lstat("sub")
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	fi, err = root.Lstat("sub/nested.txt")
	require.NoError(t, err)
	require.False(t, fi.IsDir())

	// adapter Lstat for missing path fails
	_, err = root.Lstat("missing.txt")
	require.Error(t, err)

	// adapter Open for existing file and read contents
	rc, err := root.Open("hello.txt")
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "hello real fs", string(data))
	require.NoError(t, rc.Close())

	rc, err = root.Open("sub/nested.txt")
	require.NoError(t, err)
	data, err = io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "world real fs", string(data))
	require.NoError(t, rc.Close())

	// adapter Open for missing file fails
	_, err = root.Open("missing.txt")
	require.Error(t, err)

	// realOSFs.OpenRoot on a non-existent path or a file (not a dir) fails
	_, err = fsys.OpenRoot(filepath.Join(dir, "does-not-exist"))
	require.Error(t, err)

	_, err = fsys.OpenRoot(filePath)
	require.Error(t, err)
}
