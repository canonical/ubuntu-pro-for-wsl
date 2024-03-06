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

	fooContent := "foo"

	tests := map[string]struct {
		existingLogContent string

		runError         bool
		usageErrorReturn bool
		logDirError      bool

		wantReturnCode        int
		wantOldLogFileContent *string
	}{
		"Run and exit successfully":                                {},
		"Run and exit successfully despite logs not being written": {logDirError: true},

		// Log file handling
		"Existing log file has been renamed to old": {existingLogContent: "foo", wantOldLogFileContent: &fooContent},
		"Ignore when failing to archive log file":   {existingLogContent: "OLD_IS_DIRECTORY", wantReturnCode: 0},

		// Error cases
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

			publicDir, err := a.PublicDir()
			logFile := filepath.Join(publicDir, "log")
			oldLogFile := logFile + ".old"
			if err == nil {
				switch tc.existingLogContent {
				case "":
				case "OLD_IS_DIRECTORY":
					err := os.Mkdir(oldLogFile, 0700)
					require.NoError(t, err, "Setup: create invalid log.old file")
					err = os.WriteFile(logFile, []byte("Old log content"), 0600)
					require.NoError(t, err, "Setup: creating pre-existing log file")
				default:
					err := os.WriteFile(logFile, []byte(tc.existingLogContent), 0600)
					require.NoError(t, err, "Setup: creating pre-existing log file")
				}
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

			if tc.wantOldLogFileContent != nil {
				require.FileExists(t, oldLogFile, "Old log file should exist")
				content, err := os.ReadFile(oldLogFile)
				require.NoError(t, err, "Should be able to read old log file")
				require.Equal(t, tc.existingLogContent, string(content), "Old log file content should be log's content")
			} else {
				require.NoFileExists(t, oldLogFile, "Old log file should not exist")
			}
		})
	}
}
