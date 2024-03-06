package main

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests are skipped on Windows because SIGTERM and SIGKILL
// cannot be properly captured without killing the test process.

func TestRunSignal(t *testing.T) {
	tests := map[string]struct {
		sendSig syscall.Signal

		wantReturnCode int
	}{
		// Signals handling
		"Send SIGINT exits":  {sendSig: syscall.SIGINT},
		"Send SIGTERM exits": {sendSig: syscall.SIGTERM},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Signal handlers tests: can’t be parallel

			a := myApp{
				done: make(chan struct{}),
			}

			var rc int
			wait := make(chan struct{})
			go func() {
				rc = run(&a)
				close(wait)
			}()

			time.Sleep(100 * time.Millisecond)

			var exited bool
			switch tc.sendSig {
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGTERM:
				err := syscall.Kill(syscall.Getpid(), tc.sendSig)
				require.NoError(t, err, "Teardown: kill should return no error")
				select {
				case <-time.After(50 * time.Millisecond):
					exited = false
				case <-wait:
					exited = true
				}
				require.True(t, exited, "Expect to exit on SIGINT and SIGTERM")
			}

			if !exited {
				a.Quit()
				<-wait
			}

			require.Equal(t, tc.wantReturnCode, rc, "Return expected code")
		})
	}
}
