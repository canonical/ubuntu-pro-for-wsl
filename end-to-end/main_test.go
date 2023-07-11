package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubuntu/decorate"
var wslProServiceDebPath string
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
		log.Fatalf("Setup: %v\n", err)
	}

	m.Run()
}

func assertAppxInstalled(ctx context.Context, appx string) error {
	out, err := powershellf(ctx, `(Get-AppxPackage -Name %q).Status`, appx).Output()
	if err != nil {
		return fmt.Errorf("could not determine if %q is installed: %v. %s.", appx, err, out)
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
