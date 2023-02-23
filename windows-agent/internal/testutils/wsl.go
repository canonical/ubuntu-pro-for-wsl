package testutils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows"
)

const testDistroPattern = `testDistro_UP4W_%d`

// RegisterDistro registers a distro and returns its randomly-generated name and its GUID.
func RegisterDistro(t *testing.T, realDistro bool) (distroName string, GUID windows.GUID) {
	t.Helper()

	distroName = generateDistroName(t)
	guid := registerDistro(t, distroName, realDistro)
	return distroName, guid
}

// NonRegisteredDistro generates a random distroName and GUID but does not register them.
func NonRegisteredDistro(t *testing.T) (distroName string, GUID windows.GUID) {
	t.Helper()

	distroName = generateDistroName(t)
	GUID, err := windows.GenerateGUID()
	require.NoError(t, err, "Setup: could not generate a GUID for the non-registered distro")
	return distroName, GUID
}

// UnregisterDistro unregisters a WSL distro. Errors are ignored.
func UnregisterDistro(t *testing.T, distroName string) {
	t.Helper()

	// Avoiding misuse
	var tmp int
	_, err := fmt.Sscanf(distroName, testDistroPattern, &tmp)
	require.NoErrorf(t, err, "UnregisterDistro can only be used with test distros, i.e. those that follow pattern %q", testDistroPattern)

	// Unregister distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { poweshellOutputf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()
	d := gowsl.NewDistro(distroName)
	_ = d.Unregister()
}

// ReregisterDistro unregister, then registers the same distro again.
func ReregisterDistro(t *testing.T, distroName string, realDistro bool) (GUID windows.GUID) {
	t.Helper()

	UnregisterDistro(t, distroName)
	return registerDistro(t, distroName, realDistro)
}

// DistroState returns the state of the distro as specified by wsl.exe. Possible states:
// - Installing
// - Running
// - Stopped
// - Unregistered.
func DistroState(t *testing.T, distroName string) string {
	t.Helper()

	cmd := "$env:WSL_UTF8=1 ; wsl --list --all --verbose"
	out := poweshellOutputf(t, cmd)

	rows := strings.Split(out, "\n")[1:] // [1:] to skip header
	for _, row := range rows[1:] {
		fields := strings.Fields(row)
		if fields[0] == "*" {
			fields = fields[1:]
		}
		t.Logf("Searching: %q Found:%q", distroName, fields)
		require.Len(t, fields, 3, "Output of %q should contain three columns. Row %q was parsed into %q", cmd, row, fields)
		if fields[0] != distroName {
			continue
		}
		t.Log("OK!")
		return fields[1]
	}
	return "Unregistered"
}

// generateDistroName generates a distroName that is not registered.
func generateDistroName(t *testing.T) (name string) {
	t.Helper()

	for i := 0; i < 10; i++ {
		//nolint: gosec // No need to be cryptographically secure in a distro name generator
		name := fmt.Sprintf(testDistroPattern, rand.Uint32())
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

func registerDistro(t *testing.T, distroName string, realDistro bool) (GUID windows.GUID) {
	t.Helper()
	tmpDir := t.TempDir()

	var rootFsPath string
	if !realDistro {
		rootFsPath = tmpDir + "/install.tar.gz"
		err := os.WriteFile(rootFsPath, []byte{}, 0600)
		require.NoError(t, err, "could not write empty file")
	} else {
		const appx = "UbuntuPreview"
		rootFsPath = poweshellOutputf(t, `(Get-AppxPackage | Where-Object Name -like 'CanonicalGroupLimited.%s').InstallLocation`, appx)
		require.NotEmpty(t, rootFsPath, "could not find rootfs tarball. Is %s installed?", appx)
		rootFsPath = filepath.Join(rootFsPath, "install.tar.gz")
	}

	_, err := os.Lstat(rootFsPath)
	require.NoError(t, err, "Setup: Could not stat rootFs:\n%s", rootFsPath)

	// Register distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { poweshellOutputf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()
	poweshellOutputf(t, "$env:WSL_UTF8=1 ; wsl.exe --import %q %q %q", distroName, tmpDir, rootFsPath)
	tk.Stop()

	t.Cleanup(func() {
		UnregisterDistro(t, distroName)
	})

	d := gowsl.NewDistro(distroName)
	GUID, err = d.GUID()
	require.NoError(t, err, "Setup: could not get distro GUID")

	return GUID
}

// poweshellOutputf runs the command (with any printf-style directives and args). It fails if the
// return value of the command is non-zero. Otherwise, it returns its combined stdout and stderr.
func poweshellOutputf(t *testing.T, command string, args ...any) string {
	t.Helper()

	cmd := fmt.Sprintf(command, args...)

	//nolint: gosec // This function is only used in tests so no arbitrary code execution here
	out, err := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
