//go:build !windows

package daemon_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
		return nil, errors.New("no contents configured for " + name)
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
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantContent, string(got))
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

	t.Run("Refuses a symlink rootDir", func(t *testing.T) {
		t.Parallel()

		// os.OpenRoot follows symlinks to directories without complaint, so the
		// reader must reject a symlink root at the absolute path before opening.
		target := t.TempDir()
		require.NoError(t, os.Chmod(target, 0o700)) //nolint:gosec // test exercises 0700 directory validation
		linkParent := t.TempDir()
		link := filepath.Join(linkParent, "link-to-target")
		require.NoError(t, os.Symlink(target, link))

		reader := daemon.NewDefaultSecureReader(nil)
		_, err := reader.ReadFile(link, "anything.txt")
		require.ErrorContains(t, err, "root is a symlink")
	})

	t.Run("Refuses a missing rootDir", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "does-not-exist")
		reader := daemon.NewDefaultSecureReader(nil)
		_, err := reader.ReadFile(missing, "anything.txt")
		require.ErrorContains(t, err, "could not stat")
	})
}

// TestDefaultSecureReader_RealFS_RefusesNonRootOwnership asserts the refusal path through
// the production reader and the production openRoot seam, without needing root privileges:
// every file the test creates is owned by the current user (uid != 0), so validation must
// refuse it.
func TestDefaultSecureReader_RealFS_RefusesNonRootOwnership(t *testing.T) {
	t.Parallel()

	reader := daemon.NewDefaultSecureReader(nil)

	rootDir := t.TempDir()

	// Make the root directory's mode correct (0700) so the refusal is attributable
	// to ownership, not to permissions.
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

// TestOpenRootOS_AndAdapter exercises the production openRootOS seam and the
// osRootAdapter that bridges *os.Root to the rootFs interface, bypassing the reader's
// ownership checks. This is the integration seam that the rest of the test suite relies on
// (TestDefaultSecureReader_RealFS_RefusesNonRootOwnership covers the wiring through
// validateNode; TestOpenRootOS_ConfinesPathResolution below covers confinement).
func TestOpenRootOS_AndAdapter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello real fs"), 0o600))
	subDir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	nestedPath := filepath.Join(subDir, "nested.txt")
	require.NoError(t, os.WriteFile(nestedPath, []byte("world real fs"), 0o600))

	root, err := daemon.OpenRoot(dir)
	require.NoError(t, err)
	require.NotNil(t, root)
	defer func() {
		// os.Root is single-use; further uses after Close return errors.
		require.NoError(t, root.Close())
	}()

	// adapter Lstat for existing file and directory
	fi, err := root.Lstat("hello.txt")
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

	// openRootOS on a non-existent path or a file (not a dir) fails
	_, err = daemon.OpenRoot(filepath.Join(dir, "does-not-exist"))
	require.Error(t, err)

	_, err = daemon.OpenRoot(filePath)
	require.Error(t, err)
}

// TestOpenRootOS_ConfinesPathResolution asserts the kernel-enforced confinement properties
// of os.Root on a real filesystem, through the production openRootOS seam. It deliberately
// bypasses ReadFile's ownership validation, since unprivileged test runs cannot satisfy the
// uid=0/gid=0/0700/0600 invariant. The corresponding rejection path through ReadFile is
// covered by TestDefaultSecureReader_RealFS_RefusesNonRootOwnership.
func TestOpenRootOS_ConfinesPathResolution(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		// setup populates the directory tree rooted at rootDir with whatever
		// files and symlinks the case requires and returns the root-relative
		// path the runner should pass to root.Lstat and root.Open. Returning
		// the target lets the same field type carry both static names
		// ("ok.txt", "leak.txt", "abs.txt", "/etc/passwd") and the
		// dynamically-computed ".." escape that depends on t.TempDir's chosen
		// path. t.TempDir handles teardown.
		setup func(t *testing.T, rootDir string) string

		// Lstat expectations.
		wantLstatErr  bool // Lstat must return an error.
		wantLstatLink bool // Lstat must return a FileInfo reporting a symlink.

		// Open expectations.
		wantOpenErr  bool   // Open must return an error.
		wantOpenData string // if non-empty, Open must succeed and read this content.
	}{
		"Open inside the root succeeds": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				require.NoError(t, os.WriteFile(
					filepath.Join(rootDir, "ok.txt"),
					[]byte("ok"),
					0o600,
				))
				return "ok.txt"
			},
			wantOpenData: "ok",
		},
		"Lstat and Open refuse absolute paths": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				return "/etc/passwd"
			},
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat and Open refuse .. that escapes the root": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				outside := t.TempDir()
				secret := filepath.Join(outside, "secret.txt")
				require.NoError(t, os.WriteFile(secret, []byte("outside"), 0o600))
				// filepath.Rel produces a path of the form
				// "../<tempdir-component>/secret.txt" whose leading ".."
				// asserts at runtime that the file lies outside the root.
				rel, err := filepath.Rel(rootDir, secret)
				require.NoError(t, err)
				require.True(t, strings.HasPrefix(rel, ".."), "setup: outside file must lie outside the root")
				return rel
			},
			wantLstatErr: true,
			wantOpenErr:  true,
		},
		"Lstat does not follow in-root symlinks": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				outside := t.TempDir()
				secret := filepath.Join(outside, "secret.txt")
				require.NoError(t, os.WriteFile(secret, []byte("outside"), 0o600))
				require.NoError(t, os.Symlink(secret, filepath.Join(rootDir, "leak.txt")))
				return "leak.txt"
			},
			// Open is exercised here only to keep the case self-consistent
			// under the (target-escapes-root) symlink we just created; the
			// separate "Open refuses in-root symlinks pointing outside the
			// root" case carries the Open-only assertion.
			wantLstatLink: true,
			wantOpenErr:   true,
		},
		"Open refuses in-root symlinks pointing outside the root": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				outside := t.TempDir()
				secret := filepath.Join(outside, "secret.txt")
				require.NoError(t, os.WriteFile(secret, []byte("outside"), 0o600))
				require.NoError(t, os.Symlink(secret, filepath.Join(rootDir, "leak.txt")))
				return "leak.txt"
			},
			wantOpenErr: true,
		},
		"Open refuses symlinks with absolute targets": {
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(rootDir, "abs.txt")))
				return "abs.txt"
			},
			wantOpenErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			path := tc.setup(t, rootDir)

			root, err := daemon.OpenRoot(rootDir)
			require.NoError(t, err)
			t.Cleanup(func() { _ = root.Close() })

			switch {
			case tc.wantLstatErr:
				_, err = root.Lstat(path)
				require.Error(t, err)
			case tc.wantLstatLink:
				fi, err := root.Lstat(path)
				require.NoError(t, err)
				require.NotZero(t, fi.Mode()&fs.ModeSymlink,
					"Lstat must report the symlink itself, not its target")
			default:
				_, err = root.Lstat(path)
				require.NoError(t, err)
			}

			switch {
			case tc.wantOpenErr:
				_, err = root.Open(path)
				require.Error(t, err)
			case tc.wantOpenData != "":
				f, err := root.Open(path)
				require.NoError(t, err)
				t.Cleanup(func() { _ = f.Close() })

				data, err := io.ReadAll(f)
				require.NoError(t, err)
				require.Equal(t, tc.wantOpenData, string(data))
			default:
				_, err = root.Open(path)
				require.NoError(t, err)
			}
		})
	}
}
