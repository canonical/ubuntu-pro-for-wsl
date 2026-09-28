package testutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// NewFakeFs mounts a FUSE filesystem at mountpoint under which every file and
// directory appears owned by uid=0, gid=0 with 0700/0600 permissions to any
// reader. Writes through the mountpoint are stored in a private backing
// directory owned by the test runner, so the test process itself needs no
// extra privileges.
//
// This makes the daemon's SecureReader accept test files whose real ownership
// and mode are the test runner's: the test process is unprivileged, but every
// stat() the kernel serves against the mountpoint returns the Secure
// Projection contract (root ownership, 0700 directories, 0600 files).
//
// On first use, mountpoint must exist and be empty; its backing directory is
// created under that test's t.TempDir().
//
// NewFakeFs is idempotent per canonical mountpoint across the whole process:
// FUSE is a process-wide resource, and re-mounting the same path would shadow
// the first mount and leak it. Reconnection-style tests legitimately recreate a
// mock agent on an already-mounted public directory within the same test, so
// subsequent calls for the same path return nil and keep the original backing
// directory and cleanup.
func NewFakeFs(t *testing.T, mountpoint string) (err error) {
	t.Helper()

	key, err := filepath.Abs(mountpoint)
	if err != nil {
		return fmt.Errorf("resolve mountpoint: %v", err)
	}

	fakeFsMu.Lock()
	if _, mounted := fakeFsMounts[key]; mounted {
		fakeFsMu.Unlock()
		return nil
	}
	// Reserve the key before mounting so a second call for the same path (which
	// happens when a test restarts a mock agent) sees "already mounted" instead
	// of racing with the in-progress mount or shadowing it.
	fakeFsMounts[key] = struct{}{}
	fakeFsMu.Unlock()

	// The reservation must be undone if we fail before the mount is live. On
	// success it is kept until the t.Cleanup below runs at test end.
	defer func() {
		if err != nil {
			fakeFsMu.Lock()
			delete(fakeFsMounts, key)
			fakeFsMu.Unlock()
		}
	}()

	if err := os.MkdirAll(mountpoint, 0700); err != nil {
		return fmt.Errorf("create mountpoint: %v", err)
	}
	entries, err := os.ReadDir(mountpoint)
	if err != nil {
		return fmt.Errorf("read mountpoint: %v", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("mountpoint %q is not empty (%d entries); cannot mount FUSE on top", mountpoint, len(entries))
	}

	backing := t.TempDir()
	root, err := newFakeOwnerRoot(backing)
	if err != nil {
		return fmt.Errorf("build FUSE root: %v", err)
	}

	zero := 0 * time.Second
	server, err := fs.Mount(mountpoint, root, &fs.Options{
		AttrTimeout:  &zero, // no attribute caching, always ask GetAttr.
		EntryTimeout: &zero,
		MountOptions: fuse.MountOptions{
			FsName: "fakeownerfs",
			Name:   "fakeownerfs",
		},
	})
	if err != nil {
		return fmt.Errorf("FUSE mount: %v", err)
	}

	t.Cleanup(func() {
		if err := server.Unmount(); err != nil && !errors.Is(err, syscall.EINVAL) {
			t.Logf("NewFakeFs: unmount %q: %v", mountpoint, err)
		}
		server.Wait()

		fakeFsMu.Lock()
		delete(fakeFsMounts, key)
		fakeFsMu.Unlock()
	})

	return nil
}

// fakeFsMounts tracks mountpoints that NewFakeFs has already provisioned. FUSE
// mounts are process-wide and cannot be stacked onto the same path, so this
// makes NewFakeFs a once-per-path operation while the test binary lives.
var (
	fakeFsMu     sync.Mutex
	fakeFsMounts = make(map[string]struct{})
)

// fakeOwnerNode embeds *fs.LoopbackNode so the loopback machinery continues
// to handle LOOKUP/OPEN/READ/WRITE/etc., and overrides Getattr to present the
// Secure Projection contract. Ownership and permissions are rewritten to what
// the daemon's SecureReader requires (uid=0, gid=0; 0700 for directories, 0600
// for files), regardless of the real backing tree's ownership and mode.
type fakeOwnerNode struct {
	*fs.LoopbackNode
}

// contractMode forces the permission bits to the Secure Projection contract:
// directories must be 0700 and regular files 0600, preserving the file type
// bits and any special types untouched.
func contractMode(mode uint32) uint32 {
	const permMask = uint32(0o777)
	switch mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		return (mode &^ permMask) | 0o700
	case syscall.S_IFREG:
		return (mode &^ permMask) | 0o600
	default:
		return mode
	}
}

// Getattr populates out from the real underlying file, then rewrites uid/gid
// (to 0) and permission bits (to the Secure Projection contract) so the
// daemon's SecureReader accepts the file.
func (n *fakeOwnerNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	errno := n.LoopbackNode.Getattr(ctx, fh, out)
	if errno != 0 {
		return errno
	}
	out.Uid = 0
	out.Gid = 0
	out.Mode = contractMode(out.Mode)
	return 0
}

var _ fs.NodeGetattrer = (*fakeOwnerNode)(nil)

// newFakeOwnerRoot builds the FUSE root node that mirrors backing but reports
// uid=0, gid=0 for every file.
//
// LoopbackNode.Lookup is hard-coded to call LoopbackRoot.newNode(), an
// unexported method that delegates to LoopbackRoot.NewNode. There is no
// NodeWrapChilder path inside LoopbackNode itself. We therefore set NewNode
// (deprecated in the public API but still wired through to all LOOKUP
// responses) to wrap each child in a fakeOwnerNode so the uid=0,gid=0 lie
// applies to every file the kernel stats under the mountpoint.
//
//nolint:staticcheck // SA1019: NewNode is the only path that intercepts LoopbackNode.Lookup. The preferred NodeWrapChilder interface is not consulted by Lookup().
func newFakeOwnerRoot(backing string) (fs.InodeEmbedder, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(backing, &st); err != nil {
		return nil, fmt.Errorf("stat backing dir: %v", err)
	}

	lr := &fs.LoopbackRoot{
		Path: backing,
		Dev:  st.Dev,
	}

	lr.NewNode = func(rootData *fs.LoopbackRoot, _ *fs.Inode, _ string, _ *syscall.Stat_t) fs.InodeEmbedder {
		return &fakeOwnerNode{LoopbackNode: &fs.LoopbackNode{RootData: rootData}}
	}

	// Replicate fs.NewLoopbackRoot: the unexported newNode call yields the
	// root inode, which we register on the LoopbackRoot.
	rootNode := lr.NewNode(lr, nil, "", &st)
	lr.RootNode = rootNode
	return rootNode, nil
}

// ErrFUSEUnavailable is returned by NewFakeFs on non-Linux platforms. Tests
// should t.Skipf on this.
var ErrFUSEUnavailable = errors.New("FUSE is unavailable on this platform")
