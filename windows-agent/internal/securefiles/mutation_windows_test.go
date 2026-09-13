//go:build windows

// Windows-only: a concurrent reader holding the file open with full sharing
// (including FILE_SHARE_DELETE) must never observe a partially written file
// during the custodian's atomic rename. Not reproducible on Linux, where
// rename semantics and open locking differ.

package securefiles_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestAtomicWriteConcurrentReader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := securefiles.Open(dir)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	initialContent := bytes.Repeat([]byte("A"), 1024*1024)
	updatedContent := bytes.Repeat([]byte("B"), 1024*1024)

	err = c.WriteFile("data.bin", initialContent)
	require.NoError(t, err)

	stop := make(chan struct{})
	var readerErr error
	var wg sync.WaitGroup

	wg.Go(func() {
		target := filepath.Join(dir, "data.bin")
		for {
			select {
			case <-stop:
				return
			default:
				data, err := readSharedFile(target)
				if err != nil {
					time.Sleep(time.Millisecond)
					continue
				}
				if len(data) > 0 && !bytes.Equal(data, initialContent) && !bytes.Equal(data, updatedContent) {
					readerErr = os.ErrInvalid
					return
				}
			}
		}
	})

	time.Sleep(10 * time.Millisecond)
	err = c.WriteFile("data.bin", updatedContent)
	require.NoError(t, err)

	close(stop)
	wg.Wait()
	require.NoError(t, readerErr)
}

type handleReader struct {
	h windows.Handle
}

func (r *handleReader) Read(p []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(r.h, p, &n, nil)
	if errors.Is(err, windows.ERROR_HANDLE_EOF) || (n == 0 && err == nil) {
		return int(n), io.EOF
	}
	if err != nil {
		return int(n), err
	}
	return int(n), nil
}

// readSharedFile reads a file with full sharing enabled, including
// FILE_SHARE_DELETE, so the custodian's atomic rename is not blocked while the
// reader holds the file open. Go's os.Open omits FILE_SHARE_DELETE, which would
// race with the rename and make this test flaky.
func readSharedFile(path string) ([]byte, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(
		path16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer closeHandle(h)

	return io.ReadAll(&handleReader{h: h})
}
