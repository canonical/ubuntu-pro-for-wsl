package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
)

//nolint:revive // The context is better after the testing.T
func powershellInstallDistro(t *testing.T, ctx context.Context, distroName string, realDistro bool) (GUID string) {
	t.Helper()
	tmpDir := t.TempDir()

	var rootFsPath string
	if !realDistro {
		rootFsPath = tmpDir + "/install.tar.gz"
		err := os.WriteFile(rootFsPath, []byte{}, 0600)
		require.NoError(t, err, "could not write empty file")
	} else {
		const appx = "UbuntuPreview"
		rootFsPath = powershellOutputf(t, `(Get-AppxPackage | Where-Object Name -like 'CanonicalGroupLimited.%s').InstallLocation`, appx)
		require.NotEmpty(t, rootFsPath, "could not find rootfs tarball. Is %s installed?", appx)
		rootFsPath = filepath.Join(rootFsPath, "install.tar.gz")
	}

	_, err := os.Lstat(rootFsPath)
	require.NoError(t, err, "Setup: Could not stat rootFs:\n%s", rootFsPath)

	// Register distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { powershellOutputf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()
	powershellOutputf(t, "$env:WSL_UTF8=1 ; wsl.exe --import %q %q %q", distroName, tmpDir, rootFsPath)
	tk.Stop()

	t.Cleanup(func() {
		UnregisterDistro(t, ctx, distroName)
	})

	d := gowsl.NewDistro(ctx, distroName)
	guid, err := d.GUID()
	require.NoError(t, err, "Setup: could not get distro GUID")

	return guid.String()
}

// powershellOutputf runs the command (with any printf-style directives and args). It fails if the
// return value of the command is non-zero. Otherwise, it returns its combined stdout and stderr.
func powershellOutputf(t *testing.T, command string, args ...any) string {
	t.Helper()

	cmd := fmt.Sprintf(command, args...)

	//nolint:gosec // This function is only used in tests so no arbitrary code execution here
	out, err := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
