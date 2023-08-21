package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Obfuscate returns a partially hidden version of the contents, suitable for logging low-sensitive information.
// Hidden enough to prevent others from reading the value while still allowing the contents author to recognize it.
// Useful for reading logs with test data. For example: `Obfuscate("Blahkilull")=="Bl******ll`".
func Obfuscate(contents string) string {
	const endsToReveal = 2
	asterisksLength := len(contents) - 2*endsToReveal
	if asterisksLength < 1 {
		return strings.Repeat("*", len(contents))
	}

	return contents[0:endsToReveal] + strings.Repeat("*", asterisksLength) + contents[asterisksLength+endsToReveal:]
}

// WSLLauncher translates the name of an Ubuntu WSL distro into the base path for its launcher.
func WSLLauncher(distroName string) (string, error) {
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
	case "ubuntupreview.exe":
		return executable, nil
	default:
		if regexp.MustCompile(`^ubuntu\d\d\d\d\.exe$`).MatchString(executable) {
			return executable, nil
		}
	}

	return "", fmt.Errorf("WSL launcher executable %q does not match expected pattern", executable)
}

// FindWorkspaceRoot climbs up the current working directory until the Go workspace root is found.
func FindWorkspaceRoot() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", errors.New("could not get current working directory")
	}

	for {
		parent := filepath.Dir(path)
		if parent == path {
			// Reached filesystem root
			return "", errors.New("could not find repository root")
		}
		path = parent

		if s, err := os.Stat(filepath.Join(path, "go.work")); err == nil && !s.IsDir() {
			return parent, nil
		}
	}
}
