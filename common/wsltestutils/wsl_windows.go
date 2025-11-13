package wsltestutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

// PowershellImportDistro uses powershell.exe to import a distro from a specific rootfs.
// If the rootfs is an empty string, an empty tarball will be used.
//
//nolint:revive // The context is better after the testing.T
func PowershellImportDistro(t *testing.T, ctx context.Context, distroName string, rootFsPath string) (GUID string) {
	t.Helper()
	tmpDir := t.TempDir()

	require.False(t, wsl.MockAvailable(), "Called PowershellImportDistro with the gowslmock active. Use RegisterDistro for a generic implementation")

	// Fake rootfs: the distro can be registered but won't run
	if rootFsPath == "" {
		rootFsPath = tmpDir + "/install.tar.gz"
		err := os.WriteFile(rootFsPath, []byte{}, 0600)
		require.NoError(t, err, "could not write empty file")
	}

	_, err := os.Lstat(rootFsPath)
	require.NoError(t, err, "Setup: Could not stat rootFs:\n%s", rootFsPath)

	// Register distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { powershellOutputf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()

	var vhdx string
	if strings.HasSuffix(rootFsPath, ".vhdx") {
		vhdx = "--vhd"
	}

	powershellOutputf(t, "$env:WSL_UTF8=1 ; wsl.exe --import %q %q %q %s", distroName, tmpDir, rootFsPath, vhdx)
	tk.Stop()

	t.Cleanup(func() {
		UnregisterDistro(t, ctx, distroName)
	})

	d := wsl.NewDistro(ctx, distroName)
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
	out, err := exec.Command("powershell", "-NoProfile", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
