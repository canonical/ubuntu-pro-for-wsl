package distro_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distro"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestNew(t *testing.T) {
	setLogger(t, log.DebugLevel)

	realDistro, realGUID := registerDistro(t, false)

	fakeDistro := generateDistroName(t)
	fakeGUID, err := windows.GUIDFromString(`{12345678-1234-1234-1234-123456789ABC}`)
	require.NoError(t, err, "Setup: could not construct fake GUID")

	props := distro.Properties{
		DistroID:    "ubuntu",
		VersionID:   "100.04",
		PrettyName:  "Ubuntu 100.04.0 LTS",
		ProAttached: true,
	}

	testCases := map[string]struct {
		distro   string
		withGUID windows.GUID

		wantErr bool
	}{
		"real distro":             {distro: realDistro},
		"real distro, real GUID":  {distro: realDistro, withGUID: realGUID},
		"real distro, wrong GUID": {distro: realDistro, withGUID: fakeGUID, wantErr: true},
		"fake distro":             {distro: fakeDistro, wantErr: true},
		"fake distro, real GUID":  {distro: fakeDistro, withGUID: realGUID, wantErr: true},
		"fake distro, wrong GUID": {distro: fakeDistro, withGUID: fakeGUID, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var d *distro.Distro
			var err error

			nilGUID := windows.GUID{}
			if tc.withGUID == nilGUID {
				d, err = distro.New(tc.distro, props)
			} else {
				d, err = distro.New(tc.distro, props, distro.WithGUID(tc.withGUID))
			}

			if err == nil {
				defer d.Cleanup(context.Background())
			}

			if tc.wantErr {
				require.Error(t, err, "Unexpected success constructing distro")
				require.ErrorIs(t, err, &distro.NotExistError{})
				return
			}

			require.Equal(t, tc.distro, d.Name, "Unexpected mismatch in distro name")
			require.Equal(t, realGUID.String(), d.GUID.String(), "Unexpected mismatch in distro GUID")
			require.Equal(t, props, d.Properties, "Unexpected mismatch in distro properties")
		})
	}
}

func TestString(t *testing.T) {
	name, guid := registerDistro(t, false)
	d, err := distro.New(name, distro.Properties{}, distro.WithGUID(guid))
	require.NoError(t, err, "unexpected error in distro.New")

	s := d.String()
	require.Contains(t, s, name, "Distro String does not show the name of the distro")
	require.Contains(t, s, strings.ToLower(guid.String()), "Distro String does not show the GUID of the distro")
}

func TestTaskProcessing(t *testing.T) {
	setLogger(t, log.DebugLevel)

	testCases := map[string]struct {
		earlyUnregister        bool // Triggers error in trying to get distro in keepAwake
		taskError              bool // Causes the task to always return an error
		forceConnectionTimeout bool // Cancels the timeout in task processing (real timeout is one minute; too long)

		// Will not be executed if an error is triggered in another goroutine
		wantNExecutions int
	}{
		"happy path":          {wantNExecutions: 1},
		"unregistered distro": {earlyUnregister: true, wantNExecutions: 0},
		"connection timeout":  {forceConnectionTimeout: true, wantNExecutions: 0},
		"erroneous task":      {taskError: true, wantNExecutions: testTaskMaxRetries},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			distroName, _ := registerDistro(t, true)
			d, err := distro.New(distroName, distro.Properties{})
			require.NoError(t, err, "Could not create distro")
			defer d.Cleanup(ctx)

			port := requireServeUnimplementedWSLServer(t, ctx, "Setup: could not serve")
			conn := requireNewConnection(t, ctx, port)

			if tc.earlyUnregister {
				unregisterDistro(t, distroName)
			}

			// Submit a task and wait for slightly more than a second (1 second is the refresh rate)
			task := &testTask{}
			if tc.taskError {
				task.Returns = errors.New("error made on purpose")
			}
			require.NoError(t, d.SubmitTask(task), "Could not submit task")
			time.Sleep(1200 * time.Millisecond)

			// Testing task without an active connection
			require.Equal(t, nil, d.Client(), "Client should have returned nil when there is no connection")
			require.Equal(t, 0, task.NExecutions, "Task unexpectedly executed without a connection")

			if tc.forceConnectionTimeout {
				d.Cleanup(ctx)
				require.Equal(t, 0, task.NExecutions, "Task unexpectedly executed without a connection")
				return
			}

			// Testing task with with active connection
			d.SetConnection(conn)
			require.NotEqual(t, nil, d.Client(), "Client should not have returned nil when there is a connection")

			// Wait for slightly more than a second (1 second is the refresh rate)
			time.Sleep(1200 * time.Millisecond)
			require.Equal(t, tc.wantNExecutions, task.NExecutions, "Task executed an unexpected amount of times after establishing a connection")

			// Testing task without with a cleaned up distro
			d.Cleanup(ctx)

			task = &testTask{}
			require.NoError(t, d.SubmitTask(task), "Could not submit task")
			require.Equal(t, 0, task.NExecutions, "Task unexpectedly executed after distro was cleaned up")
		})
	}

}

func setLogger(t *testing.T, setLvl log.Level) {
	lvl := log.GetLevel()
	log.SetLevel(setLvl)
	t.Cleanup(func() { log.SetLevel(lvl) })
}

// nolint: revive
// Putting the context in front of the testing.T would be a sacrilege.
func requireServeUnimplementedWSLServer(t *testing.T, ctx context.Context, msg string) (port uint16) {
	t.Helper()

	server := grpc.NewServer()

	lis, err := net.Listen("tcp4", "localhost:")
	require.NoErrorf(t, err, "%s: could not listen.", msg)

	wslserviceapi.RegisterWSLServer(server, &wslserviceapi.UnimplementedWSLServer{})
	go func() {
		err := server.Serve(lis)
		if err != nil {
			t.Logf("server.Serve returned non-nil error: %v", err)
		}
	}()

	onCleanupOrCancel(t, ctx, func() { server.Stop() })

	t.Logf("Started listening at %q", lis.Addr())

	fields := strings.Split(lis.Addr().String(), ":")
	portTmp, err := strconv.ParseUint(fields[len(fields)-1], 10, 16)
	require.NoError(t, err, "could not parse address")

	return uint16(portTmp)
}

// nolint: revive
// Putting the context in front of the testing.T would be a sacrilege.
func requireNewConnection(t *testing.T, ctx context.Context, port uint16) *grpc.ClientConn {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", port)

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctxTimeout, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	require.NoError(t, err, "could not contact the grpc server at %q", addr)

	onCleanupOrCancel(t, ctx, func() { conn.Close() })

	return conn
}

// nolint: revive
// Putting the context in front of the testing.T would be a sacrilege.
func onCleanupOrCancel(t *testing.T, ctx context.Context, f func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go func() {
		<-ctx.Done()
		f()
	}()
}

const testTaskMaxRetries = 5

type testTask struct {
	NExecutions int
	Returns     error
}

func (t *testTask) Execute(context.Context, wslserviceapi.WSLClient) error {
	t.NExecutions++
	return t.Returns
}

func (t *testTask) String() string {
	return "Test task"
}

func (t *testTask) ShouldRetry() bool {
	return t.NExecutions < testTaskMaxRetries
}

// generateDistroName generates a distroName that is not registered.
func generateDistroName(t *testing.T) (name string) {
	t.Helper()

	for i := 0; i < 10; i++ {
		//nolint: gosec // No need to be cryptographically secure in a distro name generator
		name := fmt.Sprintf("testDistro_UP4W_%d", rand.Uint32())
		d := gowsl.NewDistro(name)
		collision, err := d.IsRegistered()
		require.NoError(t, err, "Setup: could not asssert if distro already exists")
		if collision {
			t.Logf("Name %s is already taken. Retrying", name)
			continue
		}
		return name
	}
	require.Fail(t, "could not generate unique distro name")
	return ""
}

func registerDistro(t *testing.T, realDistro bool) (distroName string, GUID windows.GUID) {
	t.Helper()
	tmpDir := t.TempDir()

	var rootFsPath string
	if !realDistro {
		rootFsPath = tmpDir + "/install.tar.gz"
		err := os.WriteFile(rootFsPath, []byte{}, 0600)
		require.NoError(t, err, "could not write empty file")
	} else {
		const appx = "UbuntuPreview"
		rootFsPath = requirePwshf(t, `(Get-AppxPackage | Where-Object Name -like 'CanonicalGroupLimited.%s').InstallLocation`, appx)
		require.NotEmpty(t, rootFsPath, "could not find rootfs tarball. Is %s installed?", appx)
		rootFsPath = filepath.Join(rootFsPath, "install.tar.gz")
	}

	_, err := os.Lstat(rootFsPath)
	require.NoError(t, err, "Setup: Could not stat rootFs:\n%s", rootFsPath)

	distroName = generateDistroName(t)

	// Register distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { requirePwshf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()
	requirePwshf(t, "$env:WSL_UTF8=1 ; wsl.exe --import %q %q %q", distroName, tmpDir, rootFsPath)
	tk.Stop()

	t.Cleanup(func() {
		unregisterDistro(t, distroName)
	})

	d := gowsl.NewDistro(distroName)
	GUID, err = d.GUID()
	require.NoError(t, err, "Setup: could not get distro GUID")

	return distroName, GUID
}

func unregisterDistro(t *testing.T, distroName string) {
	t.Helper()

	// Unregister distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { requirePwshf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()
	d := gowsl.NewDistro(distroName)
	d.Unregister()
}

func requirePwshf(t *testing.T, command string, args ...any) string {
	t.Helper()

	cmd := fmt.Sprintf(command, args...)

	//nolint: gosec // This function is only used in tests so no arbitrary code execution here
	out, err := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
