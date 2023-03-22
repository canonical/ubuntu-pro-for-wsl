package testutils

import (
	"os"
	"path/filepath"
	"testing"

	_ "embed"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed filesystem/os-release
	osRelease []byte

	//go:embed filesystem/resolv.conf
	resolvConf []byte
)

func MockFilesystem(t *testing.T) (rootDir string) {
	t.Helper()

	rootDir = t.TempDir()

	err := os.MkdirAll(filepath.Join(rootDir, "etc"), 0750)
	require.NoError(t, err, "Setup: could not create mock /etc/")

	err = os.WriteFile(filepath.Join(rootDir, "etc/resolv.conf"), resolvConf, 0600)
	require.NoError(t, err, "Setup: could not write mock /etc/resolv.conf")

	err = os.WriteFile(filepath.Join(rootDir, "etc/os-release"), osRelease, 0600)
	require.NoError(t, err, "Setup: could not write mock /etc/os-release")

	portDir := PortDir(rootDir)
	err = os.MkdirAll(portDir, 0750)
	require.NoErrorf(t, err, "Setup: could not create mock %s", portDir)

	return rootDir
}

func PortDir(rootDir string) string {
	return filepath.Join(rootDir, "mnt/c/Users/TestUser/AppData/Local", common.LocalAppDataDir)
}
