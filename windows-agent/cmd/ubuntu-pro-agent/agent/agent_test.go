package agent_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/cmd/ubuntu-pro-agent/agent"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
	"github.com/stretchr/testify/require"
)

func TestHelp(t *testing.T) {
	a := agent.NewForTesting(t, "", "")
	a.SetArgs("--help")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoErrorf(t, err, "Run should not return an error with argument --help. Stdout: %v", getStdout())
}

func TestCompletion(t *testing.T) {
	a := agent.NewForTesting(t, "", "")
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Completion should not start the daemon. Stdout: %v", getStdout())
}

func TestVersion(t *testing.T) {
	a := agent.NewForTesting(t, "", "")
	a.SetArgs("version")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Run should not return an error")

	out := getStdout()

	fields := strings.Fields(out)
	require.Len(t, fields, 2, "wrong number of fields in version: %s", out)

	want := "ubuntu-pro-agent"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	require.Equal(t, want, fields[0], "Wrong executable name")
	require.Equal(t, "Dev", fields[1], "Wrong version")
}

func TestNoUsageError(t *testing.T) {
	a := agent.NewForTesting(t, "", "")
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)
	err := a.Run()

	require.NoError(t, err, "Run should not return an error, stdout: %v", getStdout())
	isUsageError := a.UsageError()
	require.False(t, isUsageError, "No usage error is reported as such")
}

func TestUsageError(t *testing.T) {
	t.Parallel()

	a := agent.NewForTesting(t, "", "")
	a.SetArgs("doesnotexist")

	err := a.Run()
	require.Error(t, err, "Run should return an error, stdout: %v")
	isUsageError := a.UsageError()
	require.True(t, isUsageError, "Usage error is reported as such")
}

func TestCanQuitWhenExecute(t *testing.T) {
	t.Parallel()

	a, wait := startDaemon(t)
	defer wait()

	a.Quit()
}

func TestCanQuitTwice(t *testing.T) {
	t.Parallel()

	a, wait := startDaemon(t)
	a.Quit()
	wait()

	// second Quit after Execution should not fail
	a.Quit()
}

func TestAppCanQuitWithoutExecute(t *testing.T) {
	t.Skipf("This test is skipped because it is flaky. There is no way to guarantee Quit has been called before run.")

	t.Parallel()

	a := agent.NewForTesting(t, "", "")
	a.SetArgs()

	requireGoroutineStarted(t, a.Quit)
	err := a.Run()
	require.Error(t, err, "Should return an error")

	require.Containsf(t, err.Error(), "grpc: the server has been stopped", "Unexpected error message")
}

func TestAppRunFailsOnComponentsCreationAndQuit(t *testing.T) {
	// Trigger the error with a cache directory that cannot be created over an
	// existing file, or because the required env is empty.
	//
	// Test cannot be parallel because we override the environment

	testCases := map[string]struct {
		invalidPublicDir  bool
		invalidPrivateDir bool

		invalidLocalAppData bool
		invalidUserProfile  bool
	}{
		"Invalid private directory": {invalidPublicDir: true},
		"Invalid public directory":  {invalidPrivateDir: true},
		"Invalid LocalAppData":      {invalidLocalAppData: true},
		"Invalid UserProfile":       {invalidUserProfile: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var publicDir, privateDir string

			if tc.invalidLocalAppData {
				t.Setenv("LocalAppData", "")
			} else {
				privateDir = t.TempDir()
			}

			if tc.invalidUserProfile {
				t.Setenv("UserProfile", "")
			} else {
				publicDir = t.TempDir()
			}

			badDir := filepath.Join(t.TempDir(), "file")
			if tc.invalidPublicDir {
				publicDir = badDir
			}
			if tc.invalidPrivateDir {
				privateDir = badDir
			}

			a := agent.New(agent.WithPublicDir(publicDir), agent.WithPrivateDir(privateDir), agent.WithRegistry(registry.NewMock()))
			a.SetArgs()

			err := os.WriteFile(badDir, []byte("I'm here to break the service"), 0600)
			require.NoError(t, err, "Failed to write file")

			err = a.Run()
			require.Error(t, err, "Run should exit with an error")
			a.Quit()
		})
	}
}

func TestAppGetRootCmd(t *testing.T) {
	t.Parallel()

	a := agent.NewForTesting(t, "", "")
	require.NotNil(t, a.RootCmd(), "Returns root command")
}

func TestPublicDir(t *testing.T) {
	// Not parallel because we modify the environment

	testCases := map[string]struct {
		emptyEnv bool
		badPath  bool

		wantErr bool
	}{
		"Success providing a public directory": {},

		"Error when %UserProfile% is empty":                  {emptyEnv: true, wantErr: true},
		"Error when %UserProfile% points to an invalid path": {badPath: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.emptyEnv {
				t.Setenv("UserProfile", "")
			} else if tc.badPath {
				badPath := filepath.Join(dir, "bad_dir")
				err := os.WriteFile(badPath, []byte("test file"), 0600)
				require.NoError(t, err, "Setup: could not write file to interfere with PublicDir")
				t.Setenv("UserProfile", badPath)
			} else {
				t.Setenv("UserProfile", dir)
			}

			var a agent.App
			got, err := a.PublicDir()
			if tc.wantErr {
				require.Error(t, err, "PublicDir should have returned an error")
				return
			}

			want := filepath.Join(dir, common.UserProfileDir)

			require.NoError(t, err, "PublicDir should return no error")
			require.Equal(t, want, got, "PublicDir should have returned the path under %UserProfile%")
		})
	}
}

// requireGoroutineStarted starts a goroutine and blocks until it has been launched.
func requireGoroutineStarted(t *testing.T, f func()) {
	t.Helper()

	launched := make(chan struct{})

	go func() {
		close(launched)
		f()
	}()

	<-launched
}

// startDaemon prepares and starts the daemon in the background. The done function should be called
// to wait for the daemon to stop.
func startDaemon(t *testing.T) (app *agent.App, done func()) {
	t.Helper()

	a := agent.NewForTesting(t, "", "")
	a.SetArgs()

	// Using a channel because we cannot assert in a goroutine.
	ch := make(chan error)
	go func() {
		ch <- a.Run()
		close(ch)
	}()

	a.WaitReady()
	time.Sleep(10 * time.Second)

	return a, func() {
		require.NoError(t, <-ch, "Run should exit without any errors")
	}
}

// captureStdout capture current process stdout and returns a function to get the captured buffer.
func captureStdout(t *testing.T) func() string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err, "Setup: pipe shouldn't fail")

	orig := os.Stdout
	os.Stdout = w

	t.Cleanup(func() {
		os.Stdout = orig
		w.Close()
	})

	var out bytes.Buffer
	errch := make(chan error)
	go func() {
		_, err = io.Copy(&out, r)
		errch <- err
		close(errch)
	}()

	return func() string {
		w.Close()
		w = nil
		require.NoError(t, <-errch, "Couldn't copy stdout to buffer")

		return out.String()
	}
}
