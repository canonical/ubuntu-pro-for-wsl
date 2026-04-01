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

// PowershellInstallDistro uses powershell.exe to install a distro from a specific rootfs image.
// The distro instance is not launched yet.
// It returns the GUID of the registered distro. The distro is automatically unregistered at the end of the test.
//
//nolint:revive // The context is better after the testing.T
func PowershellInstallDistro(t *testing.T, ctx context.Context, distroName string, rootFsPath string) (GUID string) {
	t.Helper()
	tmpDir := t.TempDir()

	require.False(t, wsl.MockAvailable(), "Called PowershellInstallDistro with the gowslmock active. Use RegisterDistro for a generic implementation")

	_, err := os.Lstat(rootFsPath)
	require.NoError(t, err, "Setup: Could not stat rootFs:\n%s", rootFsPath)

	// Register distro with a two minute timeout
	tk := time.AfterFunc(2*time.Minute, func() { powershellOutputf(t, `$env:WSL_UTF8=1 ; wsl --shutdown`) })
	defer tk.Stop()

	powershellOutputf(t, "$env:WSL_UTF8=1 ; wsl.exe --install --name %q --location %q --from-file %q --no-launch", distroName, tmpDir, rootFsPath)
	tk.Stop()

	t.Cleanup(func() {
		// Any other context might be already cancelled at this point, so we need a fresh one for the cleanup.
		unctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		UnregisterDistro(t, unctx, distroName)
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

	//#nosec G204 // This function is only used in tests with controlled inputs.
	out, err := exec.Command("powershell", "-NoProfile", "-Command", cmd).CombinedOutput()
	require.NoError(t, err, "Non-zero return code for command:\n%s\nOutput:%s", cmd, out)

	// Convert to string and get rid of trailing endline
	return strings.TrimSuffix(string(out), "\r\n")
}
