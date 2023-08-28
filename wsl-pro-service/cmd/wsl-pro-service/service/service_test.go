package service_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	commontestutils "github.com/canonical/ubuntu-pro-for-windows/common/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/cmd/wsl-pro-service/service"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

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

	want := "wsl-pro-service"
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

	system, mock := testutils.MockSystemInfo(t)
	srv := testutils.MockWindowsAgent(t, ctx, mock.DefaultAddrFile())

	a, wait := startDaemon(t, mock.DefaultAddrFile(), system)
	defer wait()

	time.Sleep(time.Second)
	srv.Stop()

	a.Quit()
}

func TestCanQuitTwice(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	system, mock := testutils.MockSystemInfo(t)
	testutils.MockWindowsAgent(t, ctx, mock.DefaultAddrFile())

	a, wait := startDaemon(t, mock.DefaultAddrFile(), system)

	a.Quit()
	wait()

	require.NotPanics(t, a.Quit)
}

func TestAppCanQuitWithoutExecute(t *testing.T) {
	t.Parallel()

	t.Skipf("This test is skipped because it is flaky. There is no way to guarantee Quit has been called before run.")

	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	a.SetArgs()
	defer a.Quit()

	requireGoroutineStarted(t, a.Quit)
	err := a.Run()
	require.Error(t, err, "Should return an error")

	require.Containsf(t, err.Error(), "grpc: the server has been stopped", "Unexpected error message")
}

func TestAppRunFailsOnComponentsCreationAndQuit(t *testing.T) {
	// Trigger the error with a cache directory that cannot be created over an
	// existing file

	t.Parallel()

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

			system, mock := testutils.MockSystemInfo(t)
			addrFile := mock.DefaultAddrFile()

			resolvConf := mock.Path("/etc/resolv.conf")
			if tc.invalidResolvConfFile {
				commontestutils.ReplaceFileWithDir(t, resolvConf, "Setup: could not create directory to interfere with service")
			}

			a := service.New(service.WithAgentPortFilePath(addrFile), service.WithSystem(system))

			a.SetArgs()

			if tc.invalidDaemonCache {
				err := os.WriteFile(addrFile, []byte("I'm here to break the service"), 0600)
				require.NoError(t, err, "Failed to write file")
			}

			defer a.Quit()
			err := a.Run()
			require.Error(t, err, "Run should exit with an error")
		})
	}
}

func TestAppGetRootCmd(t *testing.T) {
	t.Parallel()

	a := service.New(service.WithAgentPortFilePath(t.TempDir()))
	require.NotNil(t, a.RootCmd(), "Returns root command")
}

func TestDefaultAddrFile(t *testing.T) {
	t.Parallel()

	type wslpathBehaviour int
	const (
		wslpathOK wslpathBehaviour = iota
		WslpathBadOutput
		wslpathErr
	)

	testCases := map[string]struct {
		wslpath wslpathBehaviour

		wantErr bool
	}{
		"Success using wslpath": {},

		"Error when wslpath errors out":           {wslpath: wslpathErr, wantErr: true},
		"Error when wslpath returns a bad output": {wslpath: WslpathBadOutput, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			system, mock := testutils.MockSystemInfo(t)
			testutils.MockWindowsAgent(t, context.Background(), mock.DefaultAddrFile())

			switch tc.wslpath {
			case wslpathOK:
			case WslpathBadOutput:
				mock.SetControlArg(testutils.WslpathBadOutput)
			case wslpathErr:
				mock.SetControlArg(testutils.WslpathErr)
			}

			// Passing no port file means the daemon has to use $env:LocalAppData
			a := service.New(service.WithSystem(system))

			a.SetArgs("-vvv")

			tk := time.AfterFunc(10*time.Second, a.Quit)
			defer func() {
				if tk.Stop() {
					a.Quit()
				}
			}()

			err := a.Run()
			if tc.wantErr {
				require.Error(t, err, "Run should have returned an error")
				return
			}
			require.NoError(t, err, "Run should have returned no errors")
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
func startDaemon(t *testing.T, addrFile string, system systeminfo.System) (app *service.App, done func()) {
	t.Helper()

	a := service.New(
		service.WithAgentPortFilePath(addrFile),
		service.WithSystem(system),
	)

	a.SetArgs("-vvv")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := a.Run()
		require.NoError(t, err, "Run should exit without any error")
	}()

	t.Cleanup(a.Quit)

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

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithCmdExeMock(t *testing.T)  { testutils.CmdExeMock(t) }
