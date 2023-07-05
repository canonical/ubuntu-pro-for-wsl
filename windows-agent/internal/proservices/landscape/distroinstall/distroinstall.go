// Package distroinstall exists to implement various utilities used by landscape that need to be mocked
// in tests. As such, the real implementations are located in the _windows files, and the mocks in the
// _gowslmock files. Use build tag gowslmock to enable the latter.
package distroinstall

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ubuntu/gowsl"
)

// InstallFromExecutable finds the executable associated with the specified distro and installs it.
func InstallFromExecutable(ctx context.Context, d gowsl.Distro) error {
	executable, err := executableName(d.Name())
	if err != nil {
		return err
	}

	if out, err := executableInstallCommand(ctx, executable); err != nil {
		return fmt.Errorf("could not run launcher: %v. %s", err, out)
	}

	return err
}

// CreateUser creates a new user with the specified details in the target distro.
func CreateUser(ctx context.Context, d gowsl.Distro, userName string, userFullName string, uid uint32) error {
	if r, err := d.IsRegistered(); err != nil {
		return err
	} else if !r {
		return errors.New("not registered")
	}

	if valid := UsernameIsValid(userName); !valid {
		return fmt.Errorf("Username %q is is not valid", userName)
	}

	// strip all punctuation or any math symbols, currency signs, dingbats, box-drawing characters, etc
	userFullName = regexp.MustCompile(`[\p{P}\p{S}]+`).ReplaceAllString(userFullName, "")

	out, err := addUserCommand(ctx, d, uid, userName, userFullName)
	if err != nil {
		return fmt.Errorf("could not run 'adduser': %v. Output: %s", err, out)
	}

	return nil
}

func executableName(distroName string) (string, error) {
	r := strings.NewReplacer(
		"-", "",
		".", "",
	)

	executable := strings.ToLower(r.Replace(distroName))
	executable = fmt.Sprintf("%s.exe", executable)

	// Validate executable name to protect ourselves from code injection

	if executable == "ubuntu.exe" {
		return executable, nil
	}

	if executable == "ubuntu-preview.exe" {
		return executable, nil
	}

	if regexp.MustCompile(`^ubuntu\d\d\d\d\.exe$`).MatchString(executable) {
		return executable, nil
	}

	return "", fmt.Errorf("executable name does not match expected pattern")
}

// UsernameIsValid returns true if the username matches the WSL regex for usernames.
func UsernameIsValid(userName string) bool {
	return regexp.MustCompile(`^[a-z][-a-z0-9_]*$`).MatchString(userName)
}
