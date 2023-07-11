package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

var wslProServiceDebPath string

const (
	registryPath = `Software\Canonical\UbuntuPro`

	// overrideSafety is an env variable that, if set, allows the tests to perform potentially destructive actions
	overrideSafety = "UP4W_TEST_OVERRIDE_DESTRUCTIVE_CHECKS"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := assertAppxInstalled(ctx, "MicrosoftCorporationII.WindowsSubsystemForLinux"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertAppxInstalled(ctx, "CanonicalGroupLimited.Ubuntu"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertAppxInstalled(ctx, "CanonicalGroupLimited.UbuntuProForWindows"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	path, err := locateWslProServiceDeb(ctx)
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	wslProServiceDebPath = path

	if err := assertCleanRegistry(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertCleanLocalAppData(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	cleanup, err := generateGoldenImage(ctx, "Ubuntu")
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	defer cleanup()

	m.Run()

	if err := cleanupRegistry(); err != nil {
		log.Printf("Cleanup: %v\n", err)
	}
}

func assertAppxInstalled(ctx context.Context, appx string) error {
	out, err := powershellf(ctx, `(Get-AppxPackage -Name %q).Status`, appx).Output()
	if err != nil {
		return fmt.Errorf("could not determine if %q is installed: %v. %s", appx, err, out)
	}
	s := strings.TrimSpace(string(out))
	if s != "Ok" {
		return fmt.Errorf("appx %q is not installed", appx)
	}

	return nil
}

func locateWslProServiceDeb(ctx context.Context) (s string, err error) {
	defer decorate.OnError(&err, "could not locate wsl-pro-service deb package")

	out, err := powershellf(ctx, `(Get-ChildItem -Path "../wsl-pro-service_*.deb").FullName`).Output()
	if err != nil {
		return "", fmt.Errorf("could not read expected location: %v. %s", err, out)
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errors.New("Wsl Pro Service is not built")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not make path %q absolute: %v", path, err)
	}

	return absPath, nil
}

func powershellf(ctx context.Context, command string, args ...any) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-NoLogo",
		"-NonInteractive",
		"-Command", fmt.Sprintf(command, args...))
}

func assertCleanLocalAppData() error {
	path := os.Getenv("LocalAppData")
	if path == "" {
		return errors.New("variable $env:LocalAppData should not be empty")
	}

	path = filepath.Join(path, "Ubuntu Pro")

	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not stat %q: %v", path, err)
	}

	if os.Getenv(overrideSafety) != "" {
		return cleanupLocalAppData()
	}

	return fmt.Errorf("Directory %q should not exist. Remove it from your machine"+
		"to agree to run this potentially destructive test.", path)
}

func cleanupLocalAppData() error {
	path := os.Getenv("LocalAppData")
	if path == "" {
		return errors.New("variable $env:LocalAppData should not be empty")
	}

	path = filepath.Join(path, "Ubuntu Pro")
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("could not clean up LocalAppData: %v", err)
	}

	return nil
}

func assertCleanRegistry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		// Key does not exist, as expected
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not open registry: %v", err)
	}

	k.Close()

	// Key exists: this is probably running outside of a clean runner
	if os.Getenv(overrideSafety) != "" {
		return cleanupRegistry()
	}

	// Protect unsuspecting users
	return fmt.Errorf(`UbuntuPro registry key should not exist. Remove it from your machine `+
		`to agree to run this potentially destructive test. It can be located at `+
		`HKEY_CURRENT_USER\%s`, registryPath)
}

func cleanupRegistry() error {
	err := registry.DeleteKey(registry.CURRENT_USER, registryPath)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not delete UbuntuPro key: %v", err)
	}

	return nil
}
