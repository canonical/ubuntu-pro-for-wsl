package agent_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/cmd/ubuntu-pro-agent/agent"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices"
	"github.com/stretchr/testify/require"
)

func TestHelp(t *testing.T) {
	changeArgs(t, "ubuntu-pro", "--help")
	a := agent.New()

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoErrorf(t, err, "Run should not return an error with argument --help. Stdout: %v", getStdout())
}

func TestCompletion(t *testing.T) {
	changeArgs(t, "ubuntu-pro", "completion", "bash")
	a := agent.New()

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Completion should not start the daemon. Stdout: %v", getStdout())
}

func TestVersion(t *testing.T) {
	changeArgs(t, "ubuntu-pro", "version")
	a := agent.New()

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Run should not return an error")

	out := getStdout()

	fields := strings.Fields(out)
	require.Len(t, fields, 2, "wrong number of fields in version: %s", out)

	require.True(t, strings.HasPrefix(out, "ubuntu-pro\t"), "Start printing daemon name")
	version := strings.TrimSpace(strings.TrimPrefix(out, "ubuntu-pro\t"))
	require.NotEmpty(t, version, "Version is printed")
}

func TestNoUsageError(t *testing.T) {
	changeArgs(t, "ubuntu-pro", "completion", "bash")
	a := agent.New()

	getStdout := captureStdout(t)
	err := a.Run()

	require.NoError(t, err, "Run should not return an error, stdout: %v", getStdout())
	isUsageError := a.UsageError()
	require.False(t, isUsageError, "No usage error is reported as such")
}

func TestUsageError(t *testing.T) {
	changeArgs(t, "ubuntu-pro", "doesnotexist")
	a := agent.New()

	err := a.Run()
	require.Error(t, err, "Run should return an error, stdout: %v")
	isUsageError := a.UsageError()
	require.True(t, isUsageError, "Usage error is reported as such")
}

func TestCanQuitWhenExecute(t *testing.T) {
	a, wait := startDaemon(t)
	defer wait()

	a.Quit()
}

func TestCanQuitTwice(t *testing.T) {
	a, wait := startDaemon(t)
	a.Quit()
	wait()

	// second Quit after Execution should
	a.Quit()
}

func TestAppCanQuitWithoutExecute(t *testing.T) {
	changeArgs(t, "ubuntu-pro")
	a := agent.New()

	requireGoroutineStarted(t, a.Quit)
	err := a.Run()
	require.Error(t, err, "Should return an error")

	require.Containsf(t, err.Error(), "grpc: the server has been stopped", "Unexpected error message")
}

func TestAppRunFailsOnComponentsCreationAndQuit(t *testing.T) {
	// Trigger the error with a cache directory that cannot be created over an
	// existing file

	testCases := map[string]struct {
		invalidProServicesCache bool
		invalidDaemonCache      bool
	}{
		"Invalid service cache": {invalidProServicesCache: true},
		"Invalid daemon cache":  {invalidDaemonCache: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			changeArgs(t, "ubuntu-pro")
			a := agent.New()
			cachedir := filepath.Join(t.TempDir(), "file")

			err := os.WriteFile(cachedir, []byte("I'm here to break the service"), 0640)
			require.NoError(t, err, "Failed to write file")

			if tc.invalidProServicesCache {
				overrideSliceAndRestore(t, agent.ProServicesOpts, proservices.WithCacheDir(cachedir))
			} else if tc.invalidDaemonCache {
				overrideSliceAndRestore(t, agent.DaemonOpts, daemon.WithCacheDir(cachedir))
			}

			err = a.Run()
			require.Error(t, err, "Run should exit with an error")
			a.Quit()
		})
	}
}

func TestAppGetRootCmd(t *testing.T) {
	t.Parallel()

	a := agent.New()
	require.NotNil(t, a.RootCmd(), "Returns root command")
}

// requireGoroutineStarted starts a goroutine and blocks until it has been launched
func requireGoroutineStarted(t *testing.T, f func()) {
	t.Helper()

	launched := make(chan struct{})

	go func() {
		close(launched)
		f()
	}()

	<-launched
}

// startDaemon prepares and start the daemon in the background. The done function should be called
// to wait for the daemon to stop.
func startDaemon(t *testing.T) (app *agent.App, done func()) {
	t.Helper()

	changeArgs(t, "ubuntu-pro")
	a := agent.New()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := a.Run()
		require.NoError(t, err, "Run should exits without any error")
	}()
	a.WaitReady()
	time.Sleep(50 * time.Millisecond)

	return a, func() {
		wg.Wait()
	}
}

// changeArgs allows changing command line arguments and restore it when the test ends.
func changeArgs(t *testing.T, args ...string) {
	t.Helper()

	orig := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = orig })
}

// captureStdout capture current process stdout and returns a function to get the captured buffer
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

func overrideSliceAndRestore[T any](t *testing.T, variable *[]T, values ...T) {
	t.Helper()

	orig := *variable

	*variable = append([]T{}, values...)

	t.Cleanup(func() {
		*variable = orig
	})
}
