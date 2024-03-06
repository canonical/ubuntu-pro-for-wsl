package main

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type myApp struct {
	tmpDir string
	done   chan struct{}

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

func (a *myApp) PublicDir() (string, error) {
	if a.tmpDir == "PUBLIC_DIR_ERROR" {
		return "", errors.New("mock error")
	}
	return a.tmpDir, nil
}

func TestRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runError         bool
		usageErrorReturn bool
		logDirError      bool

		wantReturnCode int
	}{
		"Run and exit successfully":                                {},
		"Run and exit successfully despite logs not being written": {logDirError: true},

		"Run and return error":                   {runError: true, wantReturnCode: 1},
		"Run and return usage error":             {usageErrorReturn: true, runError: true, wantReturnCode: 2},
		"Run and usage error only does not fail": {usageErrorReturn: true, runError: false, wantReturnCode: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			a := myApp{
				done:             make(chan struct{}),
				runError:         tc.runError,
				usageErrorReturn: tc.usageErrorReturn,
				tmpDir:           t.TempDir(),
			}

			if tc.logDirError {
				a.tmpDir = "PUBLIC_DIR_ERROR"
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
