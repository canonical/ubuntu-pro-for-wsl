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

	//#nosec G101 // This is an environment variable key, not the token itself.
	proTokenEnv = "UP4W_TEST_PRO_TOKEN"

	// overrideSafety is an env variable that, if set, allows the tests to perform potentially destructive actions.
	overrideSafety = "UP4W_TEST_OVERRIDE_DESTRUCTIVE_CHECKS"

	// prebuiltPath is an env variable that, if set, uses a build at a certain path instead of building the project anew.
	// The structure is expected to be:
	// └──${prebuiltPath}
	//    ├───images
	//    │   └──ubuntu.wsl
	//    ├───wsl-pro-service
	//    │   └──wsl-pro-service_*.deb
	//    └───windows-agent
	//        └──UbuntuProForWSL_*.msixbundle
	//
	prebuiltPath = "UP4W_TEST_BUILD_PATH"

	// referenceImage is the WSL distro image that will be used to generate the test image.
	referenceImage = "ubuntu.wsl"

	// up4wAppxPackage is the Ubuntu Pro for WSL package.
	up4wAppxPackage = "CanonicalGroupLimited.UbuntuPro"
)

func TestMain(m *testing.M) {
	if err := assertCleanRegistry(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	if err := assertCleanFilesystem(); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}

	buildPath := os.Getenv(prebuiltPath)
	if buildPath == "" {
		log.Fatalf(`Setup: environment variable %q is not set. It's expected to point to a directory with the following structure:
	    ${prebuiltPath}
	    ├───images
	    │   └──ubuntu.wsl
	    ├───wsl-pro-service
	    │   └──wsl-pro-service_*.deb
	    └───windows-agent
	        └──UbuntuProForWSL_*.msixbundle
	`, prebuiltPath)
	}

	if err := usePrebuiltProject(buildPath); err != nil {
		log.Fatalf("Setup: %v\n", err)
	}
	log.Printf("MSIX package located at %s", msixPath)
	log.Printf("Deb package located at %s", debPkgPath)

	ctx := context.Background()
	imagePath := filepath.Join(buildPath, "images", referenceImage)
	path, cleanup, err := generateTestImage(ctx, imagePath)
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

// powershellf is syntax sugar to run powrshell commands with formatted arguments.
func powershellf(ctx context.Context, command string, args ...any) *exec.Cmd {
	//#nosec G204,G702 // This is a test helper for which all callers have hardcoded inputs.
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

// generateTestImage fails if there is a distro instance named "Reference", unless the safety checks are overridden,
// in which case it's removed. The source image is then registered, transformed, exported and unregistered.
// The test image has a default user preconfigured to avoid wsl-setup from hanging on first boot.
// It also contains the wsl-pro-service Debian package found in the project build directory.
func generateTestImage(ctx context.Context, sourceImage string) (string, func(), error) {
	// #nosec G706 // sourceImage is a controlled test input, not user input.
	log.Printf("Setup: Generating test image from %q\n", sourceImage)
	reference := "Reference"

	tmpDir, err := os.MkdirTemp(os.TempDir(), "UP4W_TEST_*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Setup: Cleanup: could not remove test tempdir: %v", err)
		}
	}
	// Ensures cleanup() runs if we return early due to an error.
	defer func() {
		if err != nil {
			cleanup()
			return
		}
		// #nosec G706 // sourceImage is a controlled test input, not user input.
		log.Printf("Setup: Generated test image from %q\n", sourceImage)
	}()

	d := wsl.NewDistro(ctx, reference)
	if err := assertDistroUnregistered(d); err != nil {
		return "", nil, err
	}

	out, err := powershellf(ctx, "wsl.exe --install --name %s --from-file %s --no-launch", reference, sourceImage).CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("could not register %s instance from image %q: %v. %s", reference, sourceImage, err, out)
	}
	// #nosec G706 // sourceImage is a controlled test input, not user input.
	log.Printf("Setup: Installed %s instance from image %q\n", reference, sourceImage)

	defer func() {
		if err := d.Unregister(); err != nil {
			// #nosec G706 // sourceImage is a controlled test input, not user input.
			log.Printf("Setup: Failed to unregister %s instance from image %q after generating the test image\n", reference, sourceImage)
		}
	}()
	// Transform the reference distro instance:
	// Install wsl-pro-service from the deb package built with the project.
	out, err = powershellf(ctx, `wsl.exe -d %s -u root -- bash -ec 'apt-get update && apt-get install --yes "$(wslpath -ua %q)"'`, reference, debPkgPath).CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("could not install wsl-pro-service: %v. %s", err, out)
	}
	// Set wsl-pro-service verbosity to 3 to get more logs in case of failure. Quotes around the
	// file contents wouldn't work.
	_, _ = powershellf(ctx, `wsl.exe -d %s -u root -- bash -ec 'echo verbosity: 3 > /etc/wsl-pro-service.yaml'`, reference).CombinedOutput()
	// #nosec G706 // sourceImage and reference are variables controlled by the test, not user input.
	log.Printf("Setup: Installed wsl-pro-service into %s instance from image %q\n", reference, sourceImage)
	// Create a default admin user to prevent wsl-setup from holding the first boot.
	out, err = powershellf(ctx, `wsl.exe -d %s -u root -- adduser --quiet --gecos Ubuntu ubuntu`, reference).CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create the default user: %v\n%s", err, out)
	}
	out, err = powershellf(ctx, `wsl.exe -d %s -u root -- usermod ubuntu -aG sudo,adm`, reference).CombinedOutput()
	if err != nil {
		log.Printf("failed to turn the default user into admin: %v\n%s", err, out)
	}

	// Make sure cloud-init finds a fresh instance to initialize. Those commands should always fail, but that's still a good preventive measure.
	_, _ = powershellf(ctx, `wsl.exe -d %s -u root -- cloud-init clean --logs`, reference).CombinedOutput()
	_, _ = powershellf(ctx, `wsl.exe -d %s -u root -- rm /etc/cloud/cloud-init.disabled`, reference).CombinedOutput()

	if err := wsl.Shutdown(ctx); err != nil {
		return "", nil, fmt.Errorf("could not shut down WSL: %v", err)
	}

	path := filepath.Join(tmpDir, "snapshot.tar.gz")
	//#nosec G204 // We control the inputs.
	out, err = exec.CommandContext(ctx, "wsl.exe", "--export", reference, path).CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("could not export test image: %v. %s", err, out)
	}

	log.Println("Setup: Exported tested image")

	return path, cleanup, nil
}

func assertDistroUnregistered(d wsl.Distro) error {
	registered, err := d.IsRegistered()
	if err != nil {
		return err
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
