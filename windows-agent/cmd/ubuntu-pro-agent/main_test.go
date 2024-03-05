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
		existingLogContent string

		runError         bool
		usageErrorReturn bool
		logDirError      bool

		wantReturnCode int
		wantOldLogFile bool
	}{
		"Run and exit successfully":                                {},
		"Run and exit successfully despite logs not being written": {logDirError: true},

		// Log file handling
		"Existing log file has been renamed to old": {existingLogContent: "foo", wantOldLogFile: true},
		"Empty existing log file is overwritten":    {existingLogContent: "-", wantOldLogFile: false},

		// Error cases
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

			if tc.existingLogContent != "" {
				if tc.existingLogContent == "-" {
					tc.existingLogContent = ""
				}
				publicDir, _ := a.PublicDir()
				logFile := filepath.Join(publicDir, "log")
				err := os.WriteFile(logFile, []byte(tc.existingLogContent), 0600)
				require.NoError(t, err, "Setup: creating pre-existing log file")
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
			oldLogFile := filepath.Join(publicDir, "log.old")
			if tc.wantOldLogFile {
				require.FileExists(t, oldLogFile, "Old log file should exist")
			} else {
				require.NoFileExists(t, oldLogFile, "Old log file should not exist")
			}

			require.Equal(t, tc.wantReturnCode, rc, "Return expected code")
		})
	}
}
