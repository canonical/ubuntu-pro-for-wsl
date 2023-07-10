// Package distroinstall exists to implement various utilities used by landscape that need to be mocked
// in tests. As such, the real implementations are located in the _windows files, and the mocks in the
// _gowslmock files. Use build tag gowslmock to enable the latter.
package distroinstall

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ubuntu/decorate"
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
func CreateUser(ctx context.Context, d gowsl.Distro, userName string, userFullName string) (uid uint32, err error) {
	defer decorate.OnError(&err, "could not create user %q", userName)

	if r, err := d.IsRegistered(); err != nil {
		return 0, err
	} else if !r {
		return 0, errors.New("not registered")
	}

	if valid := UsernameIsValid(userName); !valid {
		return 0, errors.New("username is is not valid")
	}

	// strip any punctuation or any math symbols, currency signs, dingbats, box-drawing characters, etc
	userFullName = regexp.MustCompile(`[\p{P}\p{S}]+`).ReplaceAllString(userFullName, "")

	if out, err := addUserCommand(ctx, d, userName, userFullName); err != nil {
		return 0, fmt.Errorf("could not run 'adduser': %v. Output: %s", err, out)
	}

	if out, err := addUserToGroupsCommand(ctx, d, userName); err != nil {
		return 0, fmt.Errorf("could not add user to proper groups: %v. Output: %s", err, out)
	}

	if out, err := removePasswordCommand(ctx, d, userName); err != nil {
		return 0, fmt.Errorf("could not enable login: %v. Output: %s", err, out)
	}

	out, err := getUserIDCommand(ctx, d, userName)
	if err != nil {
		return 0, fmt.Errorf("user id could not be retreived: %v. Output: %s", err, out)
	}

	id64, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not parse uid %q: %v", string(out), err)
	}

	return uint32(id64), nil
}

func executableName(distroName string) (string, error) {
	r := strings.NewReplacer(
		"-", "",
		".", "",
	)

	executable := strings.ToLower(r.Replace(distroName))
	executable = fmt.Sprintf("%s.exe", executable)

	// Validate executable name to protect ourselves from code injection
	switch executable {
	case "ubuntu.exe":
		return executable, nil
	case "ubuntu-preview.exe":
		return executable, nil
	default:
		if regexp.MustCompile(`^ubuntu\d\d\d\d\.exe$`).MatchString(executable) {
			return executable, nil
		}
	}

	return "", fmt.Errorf("executable name does not match expected pattern")
}

// UsernameIsValid returns true if the username matches the WSL regex for usernames.
func UsernameIsValid(userName string) bool {
	return regexp.MustCompile(`^[a-z][-a-z0-9_]*$`).MatchString(userName)
}
