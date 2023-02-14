package distro_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distro"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows"
)

func TestDistro(t *testing.T) {
	realDistro, realGUID := registerEmptyDistro(t)

	fakeDistro := createDistroName(t)
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

			task := &testTask{}
			d.SubmitTask(task)

			time.Sleep(100 * time.Millisecond)
			require.Equal(t, 0, task.NExecutions, "Task executed without an available WSLClient")
		})
	}
}

type testTask struct {
	NExecutions int
}

func (t *testTask) Execute(context.Context, wslserviceapi.WSLClient) error {
	return nil
}

func (t *testTask) String() string {
	return "Test task"
}

func (t *testTask) ShouldRetry() bool {
	return false
}

// createDistroName generates a distroName that is not registered.
func createDistroName(t *testing.T) (name string) {
	t.Helper()

	for i := 0; i < 10; i++ {
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

func registerEmptyDistro(t *testing.T) (distroName string, GUID windows.GUID) {
	t.Helper()
	tmpDir := t.TempDir()
	fakeRootfs := tmpDir + "/install.tar.gz"

	err := os.WriteFile(fakeRootfs, []byte{}, 0600)
	require.NoError(t, err, "could not write empty file")

	distroName = createDistroName(t)

	requirePwshf(t, "$env:WSL_UTF8=1 ; wsl.exe --import %q %q %q", distroName, tmpDir, fakeRootfs)
	d := gowsl.NewDistro(distroName)

	t.Cleanup(func() {
		_ = d.Unregister()
	})

	GUID, err = d.GUID()
	require.NoError(t, err, "Setup: could not get distro GUID")

	return distroName, GUID
}

func requirePwshf(t *testing.T, command string, args ...any) string {
	t.Helper()

	cmd := fmt.Sprintf(command, args...)

	out, err := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
