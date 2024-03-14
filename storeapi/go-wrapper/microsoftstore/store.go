package microsoftstore

import (
	"errors"
	"os"
	"path/filepath"
)

// findWorkspaceRoot climbs up the current working directory until the Go workspace root is found.
func findWorkspaceRoot() (string, error) {
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
