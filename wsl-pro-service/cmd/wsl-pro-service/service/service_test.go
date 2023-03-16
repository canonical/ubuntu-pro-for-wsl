package service_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/cmd/wsl-pro-service/service"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestHelp(t *testing.T) {
	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs("--help")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoErrorf(t, err, "Run should not return an error with argument --help. Stdout: %v", getStdout())
}

func TestCompletion(t *testing.T) {
	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Completion should not start the daemon. Stdout: %v", getStdout())
}

func TestVersion(t *testing.T) {
	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs("version")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Run should not return an error")

	out := getStdout()

	fields := strings.Fields(out)
	require.Len(t, fields, 2, "wrong number of fields in version: %s", out)

	// When running under tests, the binary is "service.test[.exe]".
	want := "service.test"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	require.Equal(t, want, fields[0], "Wrong executable name")
	require.Equal(t, "Dev", fields[1], "Wrong version")
}

func TestNoUsageError(t *testing.T) {
	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)
	err := a.Run()

	require.NoError(t, err, "Run should not return an error, stdout: %v", getStdout())
	isUsageError := a.UsageError()
	require.False(t, isUsageError, "No usage error is reported as such")
}

func TestUsageError(t *testing.T) {
	t.Parallel()

	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs("doesnotexist")

	err := a.Run()
	require.Error(t, err, "Run should return an error, stdout: %v")
	isUsageError := a.UsageError()
	require.True(t, isUsageError, "Usage error is reported as such")
}

func TestCanQuitWhenExecute(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	srv := testutils.MockWindowsAgent(t, ctx, dir)

	a, wait := startDaemon(t, dir)
	defer wait()

	time.Sleep(time.Second)
	srv.Stop()

	a.Quit()
}

func TestCanQuitTwice(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	testutils.MockWindowsAgent(t, ctx, dir)

	a, wait := startDaemon(t, dir)

	a.Quit()
	wait()

	// second Quit after Execution should
	a.Quit()
}

func TestAppCanQuitWithoutExecute(t *testing.T) {
	t.Skipf("This test is skipped because it is flaky. There is no way to guarantee Quit has been called before run.")

	t.Parallel()

	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs()

	requireGoroutineStarted(t, a.Quit)
	err := a.Run()
	require.Error(t, err, "Should return an error")

	require.Containsf(t, err.Error(), "grpc: the server has been stopped", "Unexpected error message")
}

func TestAppRunFailsOnComponentsCreationAndQuit(t *testing.T) {
	t.Parallel()
	// Trigger the error with a cache directory that cannot be created over an
	// existing file

	testCases := map[string]struct {
		invalidProServicesCache bool
		invalidResolvConfFile   bool
		invalidDaemonCache      bool
	}{
		"Invalid service cache":    {invalidProServicesCache: true},
		"Invalid resolv.conf file": {invalidResolvConfFile: true},
		"Invalid daemon cache":     {invalidDaemonCache: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			badCache := filepath.Join(t.TempDir(), "file")

			var daemonCache string
			if tc.invalidDaemonCache {
				daemonCache = badCache
			}

			dir := t.TempDir()
			resolvConf := filepath.Join(dir, "resolv.conf")
			if tc.invalidResolvConfFile {
				err := os.MkdirAll(resolvConf, 0600)
				require.NoError(t, err, "Setup: could not create resolv.conf directory to interfere with service")
			} else {
				err := os.WriteFile(resolvConf, []byte("nameserver localhost"), 0600)
				require.NoError(t, err, "Setup: could not write resolv.conf file")
			}

			a := service.New(service.WithAgentPortFilePath(daemonCache), service.WithResolvConfFilePath(resolvConf))
			a.SetArgs()

			err := os.WriteFile(badCache, []byte("I'm here to break the service"), 0600)
			require.NoError(t, err, "Failed to write file")

			err = a.Run()
			require.Error(t, err, "Run should exit with an error")
			a.Quit()
		})
	}
}

func TestAppGetRootCmd(t *testing.T) {
	t.Parallel()

	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	require.NotNil(t, a.RootCmd(), "Returns root command")
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

// startDaemon prepares and start the daemon in the background. The done function should be called
// to wait for the daemon to stop.
func startDaemon(t *testing.T, winAgentDir string) (app *service.App, done func()) {
	t.Helper()

	resolv := filepath.Join(t.TempDir(), "resolv.conf")
	err := os.WriteFile(resolv, []byte("nameserver localhost"), 0600)
	require.NoError(t, err, "Setup: could not write resolv.conf")

	a := service.New(
		service.WithAgentPortFilePath(filepath.Join(winAgentDir, common.ListeningPortFileName)),
		service.WithResolvConfFilePath(resolv),
	)

	a.SetArgs("-vvv")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := a.Run()
		require.NoError(t, err, "Run should exit without any error")
	}()
	a.WaitReady()
	time.Sleep(50 * time.Millisecond)

	return a, func() {
		wg.Wait()
	}
}

// captureStdout captures current process stdout and returns a function to get the captured buffer.
// Do NOT use in parallel tests.
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
