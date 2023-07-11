package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
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

	if err := assertWslProServiceBuilt(ctx); err != nil {
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

func assertWslProServiceBuilt(ctx context.Context) error {
	out, err := powershellf(ctx, `Test-Path "../wsl-pro-service_*.deb" -PathType Leaf`).Output()
	if err != nil {
		return fmt.Errorf("could not determine if Wsl Pro Service is built: %v. %s.", err, out)
	}
	s := strings.TrimSpace(string(out))
	if s != "True" {
		return errors.New("Wsl Pro Service is not built")
	}

	return nil
}

func powershellf(ctx context.Context, command string, args ...any) *exec.Cmd {
	return exec.Command("powershell.exe",
		"-NoProfile",
		"-NoLogo",
		"-NonInteractive",
		"-Command", fmt.Sprintf(command, args...))
}
