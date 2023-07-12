package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

var (
	// wslProServiceDebPath is the path to the wsl-pro-service .deb package.
	wslProServiceDebPath string

	// goldenImagePath is the path to the golden image.
	goldenImagePath string
)

const (
	// registryPath is the path under HKEY_CURRENT_USER in which Ubuntu Pro data is stored.
	registryPath = `Software\Canonical\UbuntuPro`

	//nolint:gosec // This is an environment variable key, not the token itself.
	proTokenKey = "UP4W_TEST_PRO_TOKEN"

	// overrideSafety is an env variable that, if set, allows the tests to perform potentially destructive actions.
	overrideSafety = "UP4W_TEST_OVERRIDE_DESTRUCTIVE_CHECKS"

	// referenceDistro is the WSL distro that will be used to generate the golden image.
	referenceDistro = "Ubuntu"

	// referenceDistro is the WSL distro that will be used to generate the golden image.
	referenceDistroAppx = "CanonicalGroupLimited.Ubuntu"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := assertAppxInstalled(ctx, "MicrosoftCorporationII.WindowsSubsystemForLinux"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertAppxInstalled(ctx, referenceDistroAppx); err != nil {
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

	path, cleanup, err := generateGoldenImage(ctx, referenceDistro)
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	defer cleanup()
	goldenImagePath = path

	m.Run()

	if err := cleanupRegistry(); err != nil {
		log.Printf("Cleanup: registry: %v\n", err)
	}
}

// assertAppxInstalled returns an error if the provided Appx is not installed.
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

// locateWslProServiceDeb locates the WSL pro service at the repository root and returns its absolute path.
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

// powershellf is syntax sugar to run powrshell commands.
func powershellf(ctx context.Context, command string, args ...any) *exec.Cmd {
	//nolint:gosec // Tainted input is acceptable because all callers have their values hardcoded.
	return exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-NoLogo",
		"-NonInteractive",
		"-Command", fmt.Sprintf(command, args...))
}

// assertCleanLocalAppData returns error if directory '%LocalAppData%/Ubuntu Pro' exists.
// If safety checks are overridden, then the directory is removed and no error is returned.
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

// cleanupLocalAppData removes directory '%LocalAppData%/Ubuntu Pro' and all its contents.
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

// assertCleanRegistry returns error if registry key 'UbuntuPro' exists.
// If safety checks are overridden, then the key is removed and no error is returned.
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

// cleanupRegistry removes registry key 'UbuntuPro'.
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

// generateGoldenImage fails if the sourceDistro is registered, unless the safety checks are overridden,
// in which case the sourceDistro is removed.
// The source distro is then registered, exported after first boot, and unregistered.
func generateGoldenImage(ctx context.Context, sourceDistro string) (path string, cleanup func(), err error) {
	log.Printf("Generating golden image from %q\n", sourceDistro)
	defer log.Printf("Generated golden image from %q\n", sourceDistro)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "UP4W_TEST_*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Cleanup: could not remove test tempdir: %v", err)
		}
	}

	d := gowsl.NewDistro(ctx, sourceDistro)
	if err := assertDistroUnregistered(d); err != nil {
		cleanup()
		return "", nil, err
	}

	launcher, err := common.WSLLauncher(sourceDistro)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	out, err := powershellf(ctx, "%s install --root --ui=none", launcher).CombinedOutput()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("could not register %q: %v. %s", sourceDistro, err, out)
	}

	defer func() {
		if err := d.Unregister(); err != nil {
			log.Printf("Failed to unregister %q after generating the golden image\n", sourceDistro)
		}
	}()

	out, err = d.Command(ctx, "exit 0").CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("distro could not be waken up: %v. %s", err, out)
	}

	//nolint:gosec // sourceDistro is validated in common.WSLLauncher. The path is randomly generated in MkdirTemp().
	out, err = exec.CommandContext(ctx, "wsl.exe", "--export", sourceDistro, filepath.Join(tmpDir, "golden.tar.gz")).CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not export golden image: %v. %s", err, out)
	}

	path = filepath.Join(tmpDir, "golden.tar.gz")
	return path, cleanup, nil
}

func assertDistroUnregistered(d gowsl.Distro) error {
	registered, err := d.IsRegistered()
	if err != nil {
		return fmt.Errorf("ubuntu-preview: %v", err)
	}

	if !registered {
		return nil
	}

	if os.Getenv(overrideSafety) == "" {
		return fmt.Errorf("distro %q should not exist. Unregister it to agree to run this potentially destructive test", d.Name())
	}

	if err := d.Unregister(); err != nil {
		return err
	}

	return nil
}
