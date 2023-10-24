package main

import (
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
		tc := tc
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

func TestRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runError         bool
		usageErrorReturn bool

		wantReturnCode int
	}{
		"Run and exit successfully":              {},
		"Run and return error":                   {runError: true, wantReturnCode: 1},
		"Run and return usage error":             {usageErrorReturn: true, runError: true, wantReturnCode: 2},
		"Run and usage error only does not fail": {usageErrorReturn: true, runError: false, wantReturnCode: 0},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			a := myApp{
				done:             make(chan struct{}),
				runError:         tc.runError,
				usageErrorReturn: tc.usageErrorReturn,
			}

			var rc int
			wait := make(chan struct{})
			go func() {
				rc = run(&a)
				close(wait)
			}()

			time.Sleep(100 * time.Millisecond)

			a.Quit()
			<-wait

			require.Equal(t, tc.wantReturnCode, rc, "Return expected code")
		})
	}
}

type myApp struct {
	done chan struct{}

	runError         bool
	usageErrorReturn bool
}

func (a *myApp) Run() error {
	<-a.done
	if a.runError {
		return errors.New("Error requested")
	}
	return nil
}

func (a myApp) UsageError() bool {
	return a.usageErrorReturn
}

func (a *myApp) Quit() {
	close(a.done)
}
