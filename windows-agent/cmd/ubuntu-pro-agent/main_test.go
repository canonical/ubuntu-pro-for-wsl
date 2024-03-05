package main

import (
	"errors"
	"os"
	"path/filepath"
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
		createLog        bool
		runError         bool
		usageErrorReturn bool
		logDirError      bool

		wantReturnCode int
	}{
		"Run and exit successfully":                                {},
		"Run and exit successfully despite logs not being written": {logDirError: true},
		"Run and exit successfully when logs already exist":        {createLog: true},

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
				tmpDir:           t.TempDir(),
			}

			if tc.logDirError {
				a.tmpDir = "PUBLIC_DIR_ERROR"
			}

			if tc.createLog {
				publicDir, _ := a.PublicDir()
				logFile := filepath.Join(publicDir, "log")
				err := os.WriteFile(logFile, []byte("test log"), 0600)
				require.NoError(t, err, "")
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

			publicDir, _ := a.PublicDir()
			oldLogfile := filepath.Join(publicDir, "log.old")
			_, err := os.Stat(oldLogfile)

			if tc.createLog {
				require.NoError(t, err, "Old log file should be created")
			} else {
				require.Error(t, err, "Old log file should not be created")
			}

			require.Equal(t, tc.wantReturnCode, rc, "Return expected code")
		})
	}
}
