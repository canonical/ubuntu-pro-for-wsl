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
	wsl "github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

var (
	// testImagePath is the path to the test image.
	testImagePath string
)

const (
	// registryPath is the path under HKEY_CURRENT_USER in which Ubuntu Pro data is stored.
	registryPath = `Software\Canonical\UbuntuPro`

	//nolint:gosec // This is an environment variable key, not the token itself.
	proTokenEnv = "UP4W_TEST_PRO_TOKEN"

	// overrideSafety is an env variable that, if set, allows the tests to perform potentially destructive actions.
	overrideSafety = "UP4W_TEST_OVERRIDE_DESTRUCTIVE_CHECKS"

	// referenceDistro is the WSL distro that will be used to generate the test image.
	referenceDistro = "Ubuntu"

	// referenceDistro is the WSL distro that will be used to generate the test image.
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

	if err := assertCleanRegistry(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertCleanLocalAppData(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	wslProServiceDebPath, err := buildProject(ctx)
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	defer func() {
		if err := os.RemoveAll(wslProServiceDebPath); err != nil {
			log.Printf("could not remove debian artifacts at %q: %v", wslProServiceDebPath, err)
		}
	}()

	if err := assertAppxInstalled(ctx, "CanonicalGroupLimited.UbuntuProForWindows"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	path, cleanup, err := generateTestImage(ctx, referenceDistro, wslProServiceDebPath)
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	defer cleanup()
	testImagePath = path

	m.Run()

	if err := cleanupRegistry(); err != nil {
		log.Printf("Cleanup: registry: %v\n", err)
	}
}

func buildProject(ctx context.Context) (debPath string, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	debPath, err = os.MkdirTemp(os.TempDir(), "WslProService")
	if err != nil {
		return "", fmt.Errorf("could not create temporary directory for debian artifacts")
	}

	jobs := map[string]*exec.Cmd{
		"Build Windows Agent":   powershellf(ctx, `..\tools\build\build-appx.ps1`),
		"Build Wsl Pro Service": powershellf(ctx, `..\tools\build\build-deb.ps1 -OutputDir %q`, debPath),
	}

	results := make(chan error)
	for name, job := range jobs {
		name := name
		job := job
		go func() {
			log.Printf("Started job: %s\n", name)

			logFile := strings.ReplaceAll(fmt.Sprintf("%s.log", name), " ", "")

			if f, err := os.Create(logFile); err != nil {
				log.Printf("could not open log file for %s", name)
				f = nil
			} else {
				job.Stdout = f
				job.Stderr = f
				defer f.Close()
			}

			if err := job.Run(); err != nil {
				cancel()
				results <- fmt.Errorf("%q: %v. Check out %q for more details", name, err, logFile)
				return
			}

			log.Printf("Finished job: %s\n", name)
			results <- nil

		}()
	}

	for range jobs {
		err = errors.Join(err, <-results)
	}

	if err != nil {
		return "", fmt.Errorf("could not build project: %v", err)
	}

	log.Println("Project built")
	return debPath, nil
}

// assertAppxInstalled returns an error if the provided Appx is not installed.
func assertAppxInstalled(ctx context.Context, appx string) error {
	out, err := powershellf(ctx, `(Get-AppxPackage -Name %q).Status`, appx).CombinedOutput()
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
func locateWslProServiceDeb(ctx context.Context, path string) (debPath string, err error) {
	defer decorate.OnError(&err, "could not locate wsl-pro-service deb package")

	out, err := powershellf(ctx, `(Get-ChildItem -Path "%s/wsl-pro-service_*.deb").FullName`, path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not read expected location: %v. %s", err, out)
	}

	debPath = strings.TrimSpace(string(out))
	if debPath == "" {
		return "", errors.New("Wsl Pro Service is not built")
	}

	debPath, err = filepath.Abs(debPath)
	if err != nil {
		return "", fmt.Errorf("could not make path %q absolute: %v", path, err)
	}

	return debPath, nil
}

// powershellf is syntax sugar to run powrshell commands.
func powershellf(ctx context.Context, command string, args ...any) *exec.Cmd {
	//nolint:gosec // Tainted input is acceptable because all callers have their values hardcoded.
	return exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-NoLogo",
		"-NonInteractive",
		"-Command", fmt.Sprintf(`$env:PsModulePath="" ; `+command, args...))
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

	return fmt.Errorf("Directory %q should not exist. Remove it from your machine "+
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

// generateTestImage fails if the sourceDistro is registered, unless the safety checks are overridden,
// in which case the sourceDistro is removed.
// The source distro is then registered, exported after first boot, and unregistered.
func generateTestImage(ctx context.Context, sourceDistro, wslProServiceDebPath string) (path string, cleanup func(), err error) {
	log.Printf("Setup: Generating test image from %q\n", sourceDistro)
	defer log.Printf("Setup: Generated test image from %q\n", sourceDistro)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "UP4W_TEST_*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Setup: Cleanup: could not remove test tempdir: %v", err)
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

	log.Printf("Setup: Installed %q\n", sourceDistro)

	defer func() {
		if err := d.Unregister(); err != nil {
			log.Printf("Setup: Failed to unregister %q after generating the test image\n", sourceDistro)
		}
	}()

	// From now on, all cleanups must be deferred because the distro
	// must be unregistered before removing the directory it is in.

	debPath, err := locateWslProServiceDeb(ctx, wslProServiceDebPath)
	if err != nil {
		return "", nil, err
	}

	out, err = d.Command(ctx, fmt.Sprintf("apt install $(wslpath -ua '%s')", debPath)).CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not install wsl-pro-service: %v. %s", err, out)
	}

	log.Printf("Setup: Installed wsl-pro-service into %q\n", sourceDistro)

	if err := wsl.Shutdown(ctx); err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not shut down WSL: %v", err)
	}

	path = filepath.Join(tmpDir, "snapshot.vhdx")
	out, err = exec.CommandContext(ctx, "wsl.exe", "--export", sourceDistro, path, "--vhd").CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not export test image: %v. %s", err, out)
	}

	log.Println("Setup: Exported image")

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
