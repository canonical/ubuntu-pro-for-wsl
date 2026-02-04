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

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	wsl "github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

var (
	// testImagePath is the path to the test image.
	testImagePath string

	// msixPath is the path to the Ubuntu Pro for WSL MSIX.
	msixPath string

	// debPkgPath is the path to the Wsl Pro Service Debian package.
	debPkgPath string
)

const (
	// registryPath is the path under HKEY_CURRENT_USER in which Ubuntu Pro data is stored.
	registryPath = `Software\Canonical\UbuntuPro`

	//nolint:gosec // This is an environment variable key, not the token itself.
	proTokenEnv = "UP4W_TEST_PRO_TOKEN"

	// overrideSafety is an env variable that, if set, allows the tests to perform potentially destructive actions.
	overrideSafety = "UP4W_TEST_OVERRIDE_DESTRUCTIVE_CHECKS"

	// prebuiltPath is an env variable that, if set, uses a build at a certain path instead of building the project anew.
	// The structure is expected to be:
	// └──${prebuiltPath}
	//    ├───wsl-pro-service
	//    │   └──wsl-pro-service_*.deb
	//    └───windows-agent
	//        └──UbuntuProForWSL_*.msixbundle
	//
	prebuiltPath = "UP4W_TEST_BUILD_PATH"

	// referenceDistro is the WSL distro that will be used to generate the test image.
	referenceDistro = "Ubuntu"

	// up4wAppxPackage is the Ubuntu Pro for WSL package.
	up4wAppxPackage = "CanonicalGroupLimited.UbuntuPro"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := assertMSIXInstalled(ctx, "MicrosoftCorporationII.WindowsSubsystemForLinux"); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertCleanRegistry(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertCleanFilesystem(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	buildPath := os.Getenv(prebuiltPath)
	if buildPath == "" {
		path, err := buildProject(ctx)
		if err != nil {
			log.Fatalf("Setup: %v\n", err)
		}
		buildPath = path
	}

	if err := usePrebuiltProject(buildPath); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	log.Printf("MSIX package located at %s", msixPath)
	log.Printf("Deb package located at %s", debPkgPath)

	path, cleanup, err := generateTestImage(ctx, referenceDistro)
	if err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	defer cleanup()
	testImagePath = path

	m.Run()

	if err := cleanupRegistry(); err != nil {
		log.Printf("Cleanup: registry: %v\n", err)
	}

	cmd := powershellf(ctx, "Get-AppxPackage -Name %q | Remove-AppxPackage", up4wAppxPackage)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Cleanup: could not remove Appx: %v: %s", err, out)
	}
}

func usePrebuiltProject(buildPath string) (err error) {
	// Locate the Appx package and store the path in global variable so that we can
	// reinstall it before every test
	result, err := globSingleResult(filepath.Join(buildPath, "windows-agent", "UbuntuProForWSL_*.msixbundle"))
	if err != nil {
		return fmt.Errorf("could not locate MSIX: %v", err)
	}

	msixPath, err = filepath.Abs(result)
	if err != nil {
		return fmt.Errorf("could not make MSIX path absolute: %v", err)
	}

	// Locate WSL-Pro-Service (it'll be installed later into the distros)
	path, err := globSingleResult(filepath.Join(buildPath, "wsl-pro-service", "wsl-pro-service_*.deb"))
	if err != nil {
		return fmt.Errorf("could not locate WSL-Pro-Service: %v", err)
	}

	debPkgPath, err = filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("could not make debian package path absolute: %v", err)
	}

	return nil
}

func buildProject(ctx context.Context) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	buildPath, err := os.MkdirTemp(os.TempDir(), "UP4W-E2E-build")
	if err != nil {
		return "", fmt.Errorf("could not create temporary directory for build artifacts")
	}

	debPath := filepath.Join(buildPath, "wsl-pro-service")
	winPath := filepath.Join(buildPath, "windows-agent")

	if err := os.MkdirAll(debPath, 0600); err != nil {
		return "", fmt.Errorf("could not create directory for WSL-Pro-Service Debian package artifacts")
	}

	if err := os.MkdirAll(winPath, 0600); err != nil {
		return "", fmt.Errorf("could not create directory for Ubuntu Pro for WSL MSIX artifacts")
	}

	jobs := map[string]*exec.Cmd{
		"Build Windows Agent":   powershellf(ctx, `..\tools\build\build-appx.ps1 -Mode end_to_end_tests -OutputDir %q`, winPath),
		"Build Wsl Pro Service": powershellf(ctx, `..\tools\build\build-deb.ps1 -OutputDir %q`, debPath),
	}

	results := make(chan error)
	for jobName, cmd := range jobs {
		go func() {
			log.Printf("Started job: %s\n", jobName)

			logPath := strings.ReplaceAll(fmt.Sprintf("%s.log", jobName), " ", "")
			if f, err := os.Create(logPath); err != nil {
				log.Printf("%s: could not open log file %q for writing", jobName, logPath)
			} else {
				cmd.Stdout = f
				cmd.Stderr = f
				defer f.Close()
			}

			if err := cmd.Run(); err != nil {
				cancel()
				results <- fmt.Errorf("%q: %v. Check out %q for more details", jobName, err, logPath)
				return
			}

			log.Printf("Finished job: %s\n", jobName)
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

	return buildPath, nil
}

// assertMSIXInstalled returns an error if the provided MSIX is not installed.
func assertMSIXInstalled(ctx context.Context, msix string) error {
	out, err := powershellf(ctx, `(Get-AppxPackage -Name %q).Status`, msix).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not determine if %q is installed: %v. %s", msix, err, out)
	}
	s := strings.TrimSpace(string(out))
	if s != "Ok" {
		return fmt.Errorf("msix %q is not installed", msix)
	}

	return nil
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

// assertCleanFilesystem returns error if directory '%LocalAppData%/Ubuntu Pro' exists.
// If safety checks are overridden, then the directory is removed and no error is returned.
func assertCleanFilesystem() error {
	if os.Getenv(overrideSafety) != "" {
		return cleanupFilesystem()
	}

	fileList, err := filesToCleanUp()
	if err != nil {
		return err
	}

	var errs error
	for _, path := range fileList {
		_, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not stat %q: %v", path, err))
			continue
		}

		errs = errors.Join(errs, fmt.Errorf("path %q should not exist. Remove it from your machine "+
			"to agree to run this potentially destructive test.", path))
	}

	return nil
}

func cleanupFilesystem() error {
	fileList, err := filesToCleanUp()
	if err != nil {
		return err
	}

	var errs error
	for _, path := range fileList {
		if err := os.RemoveAll(path); err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not clean up %s: %v", path, err))
		}
	}

	return errs
}

func filesToCleanUp() ([]string, error) {
	fileList := []struct {
		prefixEnv string
		path      string
	}{
		{prefixEnv: "LocalAppData", path: common.LocalAppDataDir},
		{prefixEnv: "UserProfile", path: common.UserProfileDir},
	}

	var out []string
	var errs error

	for _, s := range fileList {
		prefix := os.Getenv(s.prefixEnv)
		if prefix == "" {
			errs = errors.Join(errs, fmt.Errorf("variable $env:%s should not be empty", s.prefixEnv))
		}

		out = append(out, filepath.Join(prefix, s.path))
	}

	return out, errs
}

// assertCleanRegistry returns error if registry key 'UbuntuPro' exists.
// If safety checks are overridden, then the key is removed and no error is returned.
func assertCleanRegistry() error {
	if os.Getenv(overrideSafety) != "" {
		return cleanupRegistry()
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		// Key does not exist, as expected
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not open registry: %v", err)
	}

	k.Close()

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
func generateTestImage(ctx context.Context, sourceDistro string) (path string, cleanup func(), err error) {
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

	d := wsl.NewDistro(ctx, sourceDistro)
	if err := assertDistroUnregistered(d); err != nil {
		cleanup()
		return "", nil, err
	}

	// We could consider using a cached image instead of installing every time CI runs.
	// That also allows testing multiple Ubuntu releases symmetrically.
	out, err := powershellf(ctx, "wsl.exe --install -d %s --no-launch", sourceDistro).CombinedOutput()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("could not register %q: %v. %s", sourceDistro, err, out)
	}
	// Let's create a default user to avoid interactive prompts during first boot.
	out, err = powershellf(ctx, "wsl.exe -d %s -- adduser --quiet --gecos Ubuntu ubuntu", sourceDistro).CombinedOutput()
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

	out, err = d.Command(ctx, fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive bash -ec "apt update && apt install -y $(wslpath -ua '%s')"`, debPkgPath)).CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not install wsl-pro-service: %v. %s", err, out)
	}
	// Minor precaution to make sure tests will find a pristine environment.
	out, err = powershellf(ctx, `wsl.exe -d %s -- cloud-init clean --logs`, sourceDistro).CombinedOutput()
	if err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("could not install wsl-pro-service: %v. %s", err, out)
	}
	// We expect this to fail most often than not.
	_, _ = powershellf(ctx, `wsl.exe -d %s -- rm /etc/cloud/cloud-init.disabled`, sourceDistro).CombinedOutput()

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

func assertDistroUnregistered(d wsl.Distro) error {
	registered, err := d.IsRegistered()
	if err != nil {
		return fmt.Errorf("ubuntu: %v", err)
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
