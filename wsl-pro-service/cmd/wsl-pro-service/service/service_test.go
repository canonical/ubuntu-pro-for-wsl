package service_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/cmd/wsl-pro-service/service"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

func TestHelp(t *testing.T) {
	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("--help")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoErrorf(t, err, "Run should not return an error with argument --help. Stdout: %v", getStdout())
}

func TestCompletion(t *testing.T) {
	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Completion should not start the daemon. Stdout: %v", getStdout())
}

func TestVersion(t *testing.T) {
	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("version")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Run should not return an error")

	out := getStdout()

	fields := strings.Fields(out)
	require.Len(t, fields, 2, "wrong number of fields in version: %s", out)

	want := "wsl-pro-service"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	require.Equal(t, want, fields[0], "Wrong executable name")
	require.Equal(t, consts.Version, fields[1], "Wrong version")
}

func TestConfigBadArg(t *testing.T) {
	getStdout := captureStdout(t)

	filename := "wsl-pro-service.yaml"
	configPath := filepath.Join(t.TempDir(), filename)

	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("version", "--config", configPath)

	err := a.Run()
	out := getStdout()
	require.Error(t, err, "Run should return an error, stdout: %v", out)
}

func TestConfigArg(t *testing.T) {
	getStdout := captureStdout(t)

	filename := "wsl-pro-service.yaml"
	configPath := filepath.Join(t.TempDir(), filename)
	require.NoError(t, os.WriteFile(configPath, []byte("verbosity: 1"), 0600), "Setup: couldn't write config file")

	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("version", "--config", configPath)

	err := a.Run()
	out := getStdout()
	require.NoError(t, err, "Run should not return an error, stdout: %v", out)
	require.Equal(t, 1, a.Config().Verbosity)
}

func TestConfigAutoDetect(t *testing.T) {
	getStdout := captureStdout(t)

	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("version")

	filename := "wsl-pro-service.yaml"
	configNextToBinaryPath := filepath.Join(filepath.Dir(os.Args[0]), filename)
	require.NoError(t, os.WriteFile(configNextToBinaryPath, []byte("verbosity: 3"), 0600), "Setup: couldn't write config file")

	err := a.Run()
	out := getStdout()
	require.NoError(t, err, "Run should not return an error, stdout: %v", out)
	require.Equal(t, 3, a.Config().Verbosity)
}

func TestNoUsageError(t *testing.T) {
	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
	a.SetArgs("completion", "bash")

	getStdout := captureStdout(t)

	err := a.Run()
	require.NoError(t, err, "Run should not return an error, stdout: %v", getStdout())

	isUsageError := a.UsageError()
	require.False(t, isUsageError, "No usage error is reported as such")
}

func TestUsageError(t *testing.T) {
	t.Parallel()

	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
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

	system, mock := testutils.MockSystem(t)
	agent := testutils.NewMockWindowsAgent(t, ctx, mock.DefaultPublicDir())
	defer agent.Stop()

	a, wait := startDaemon(t, system)
	defer wait()

	time.Sleep(time.Second)
	agent.Stop()

	a.Quit()
}

func TestCanQuitTwice(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system, mock := testutils.MockSystem(t)
	agent := testutils.NewMockWindowsAgent(t, ctx, mock.DefaultPublicDir())
	defer agent.Stop()

	a, wait := startDaemon(t, system)

	a.Quit()
	wait()

	require.NotPanics(t, a.Quit)
}

func TestAppCanQuitWithoutExecute(t *testing.T) {
	t.Parallel()

	t.Skipf("This test is skipped because it is flaky. There is no way to guarantee Quit has been called before run.")

	sys, _ := testutils.MockSystem(t)

	a := service.New(service.WithSystem(sys))
	a.SetArgs()
	defer a.Quit()

	requireGoroutineStarted(t, a.Quit)

	err := a.Run()
	require.Error(t, err, "Should return an error")

	require.Containsf(t, err.Error(), "grpc: the server has been stopped", "Unexpected error message")
}

func TestAppRunFailsOnComponentsCreationAndQuit(t *testing.T) {
	// Trigger the error with a broken wslinfo binary
	t.Parallel()

	sys, mock := testutils.MockSystem(t)
	mock.SetControlArg(testutils.WslInfoErr)

	a := service.New(service.WithSystem(sys))

	agent := testutils.NewMockWindowsAgent(t, context.Background(), mock.DefaultPublicDir())
	defer agent.Stop()

	a.SetArgs()

	defer a.Quit()
	err := a.Run()
	require.Error(t, err, "Run should exit with an error")
}

func TestAppGetRootCmd(t *testing.T) {
	t.Parallel()

	sys, _ := testutils.MockSystem(t)
	a := service.New(service.WithSystem(sys))
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

// startDaemon prepares and starts the daemon in the background. The done function should be called
// to wait for the daemon to stop.
func startDaemon(t *testing.T, s *system.System) (app *service.App, done func()) {
	t.Helper()

	a := service.New(service.WithSystem(s))

	a.SetArgs("-vvv")

	// Using a channel because we cannot assert in a goroutine.
	ch := make(chan error)
	go func() {
		ch <- a.Run()
		close(ch)
	}()

	t.Cleanup(a.Quit)

	a.WaitReady()
	time.Sleep(50 * time.Millisecond)

	return a, func() {
		require.NoError(t, <-ch, "Run should exit without any error")
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

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T) { testutils.WslInfoMock(t) }
