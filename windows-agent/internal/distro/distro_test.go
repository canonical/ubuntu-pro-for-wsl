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
	"sync/atomic"
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

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {

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

func TestIsValid(t *testing.T) {
	distro1, guid1 := registerDistro(t, false)
	_, guid2 := registerDistro(t, false)

	nonRegisteredDistro := generateDistroName(t)
	fakeGUID, err := windows.GUIDFromString(`{12345678-1234-1234-1234-123456789ABC}`)
	require.NoError(t, err, "Setup: could not construct fake GUID")

	testCases := map[string]struct {
		distro string
		guid   windows.GUID

		want bool
	}{
		"registered distro with matching GUID": {distro: distro1, guid: guid1, want: true},

		// Invalid cases
		"registered distro with diferent, another distro's GUID": {distro: distro1, guid: guid2, want: false},
		"registered distro with diferent, fake GUID":             {distro: distro1, guid: fakeGUID, want: false},
		"non-registered distro, registered distro's GUID":        {distro: nonRegisteredDistro, guid: guid1, want: false},
		"non-registered distro, non-registered distro's GUID":    {distro: nonRegisteredDistro, guid: fakeGUID, want: false},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Create an always valid distro
			d, err := distro.New(distro1, distro.Properties{})
			require.NoError(t, err, "Setup: distro New() should return no errors")

			// Change values and assert on IsValid
			d.Name = tc.distro
			d.GUID = tc.guid

			got, err := d.IsValid()
			require.NoError(t, err, "IsValid should never return an error")

			require.Equal(t, tc.want, got, "IsValid should return expected value")
		})
	}
}

func TestTaskProcessing(t *testing.T) {
	reusableDistro, _ := registerDistro(t, true)

	testCases := map[string]struct {
		earlyUnregister        bool // Triggers error in trying to get distro in keepAwake
		taskError              bool // Causes the task to always return an error
		forceConnectionTimeout bool // Cancels the while waiting for the client
		cancelTaskInProgress   bool // Cancels as the task is running

		wantExecuteCalls int32
	}{
		"happy path":              {wantExecuteCalls: 1},
		"unregistered distro":     {earlyUnregister: true, wantExecuteCalls: 0},
		"connection timeout":      {forceConnectionTimeout: true, wantExecuteCalls: 0},
		"cancel task in progress": {cancelTaskInProgress: true, wantExecuteCalls: 1},
		"erroneous task":          {taskError: true, wantExecuteCalls: testTaskMaxRetries},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			distroName := reusableDistro
			if tc.earlyUnregister {
				distroName, _ = registerDistro(t, true)
			}

			d, err := distro.New(distroName, distro.Properties{})
			require.NoError(t, err, "Could not create distro")
			defer d.Cleanup(ctx)

			port := requireServeUnimplementedWSLServer(t, ctx, "Setup: could not serve")
			conn := requireNewConnection(t, ctx, port)

			if tc.earlyUnregister {
				unregisterDistro(t, distroName)
			}

			// Submit a task and wait for slightly more than a second (1 second is the refresh rate)
			const clientTickPeriod = 1200 * time.Millisecond

			task := &testTask{}
			if tc.taskError {
				task.Returns = errors.New("error made on purpose")
			}
			if tc.cancelTaskInProgress {
				// Long delay to ensure we catch it in the act
				task.Delay = 10 * time.Second
			}
			require.NoError(t, d.SubmitTask(task), "Could not submit task")
			time.Sleep(clientTickPeriod)

			// Testing task without an active connection
			require.Equal(t, nil, d.Client(), "Client should have returned nil when there is no connection")
			require.Equal(t, int32(0), task.ExecuteCalls.Load(), "Task unexpectedly executed without a connection")

			if tc.forceConnectionTimeout {
				d.Cleanup(ctx)
				require.Equal(t, int32(0), task.ExecuteCalls.Load(), "Task unexpectedly executed without a connection")
				return
			}

			// Testing task with with active connection
			d.SetConnection(conn)
			require.NotEqual(t, nil, d.Client(), "Client should not have returned nil when there is a connection")

			if tc.wantExecuteCalls == 0 {
				time.Sleep(2 * clientTickPeriod)
				require.Equal(t, tc.wantExecuteCalls, task.ExecuteCalls.Load(), "Task executed unexpectedly")
				return
			}

			if tc.cancelTaskInProgress {
				require.Eventuallyf(t, func() bool { return task.ExecuteCalls.Load() == tc.wantExecuteCalls },
					2*clientTickPeriod, 100*time.Millisecond, "Task was executed fewer times than expected. Expected %d and executed %d.", tc.wantExecuteCalls, task.ExecuteCalls.Load())
				d.Cleanup(ctx)
				require.Equal(t, tc.wantExecuteCalls, task.ExecuteCalls.Load(), "Task was retried after being cancelled")
				return
			}

			require.Eventuallyf(t, func() bool { return task.ExecuteCalls.Load() == tc.wantExecuteCalls },
				2*clientTickPeriod, 100*time.Millisecond, "Task was executed fewer times than expected. Expected %d and executed %d.", tc.wantExecuteCalls, task.ExecuteCalls.Load())

			time.Sleep(clientTickPeriod)
			require.Equal(t, tc.wantExecuteCalls, task.ExecuteCalls.Load(), "Task executed an unexpected amount of times after establishing a connection")

			// Saturate queue
			err = nil
			for err == nil {
				if d.QueueLen() > distro.TaskQueueBufferSize+20 { // +20 to protect from races
					break
				}
				// Delayed task to avoid pulling tasks as they are added
				err = d.SubmitTask(&testTask{Delay: time.Second})
			}
			require.Error(t, err, "queue never saturated despite filling it ")
			d.FlushTaskQueue()

			// Testing task without with a cleaned up distro
			d.Cleanup(ctx)

			task = &testTask{}
			require.NoError(t, d.SubmitTask(task), "Could not submit task")
			require.Equal(t, int32(0), task.ExecuteCalls.Load(), "Task unexpectedly executed after distro was cleaned up")
		})
	}
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
	// ExecuteCalls counts the number of times Execute is called
	ExecuteCalls atomic.Int32

	// Delay simulates a processing time for the task
	Delay time.Duration

	// Returns is the value that Execute will return
	Returns error
}

func (t *testTask) Execute(ctx context.Context, _ wslserviceapi.WSLClient) error {
	t.ExecuteCalls.Add(1)
	select {
	case <-time.After(t.Delay):
		return t.Returns
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *testTask) String() string {
	return "Test task"
}

func (t *testTask) ShouldRetry() bool {
	return t.ExecuteCalls.Load() < testTaskMaxRetries
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
