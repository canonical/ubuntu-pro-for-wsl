package microsoftstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func locateStoreDll() (string, error) {
	locators := []func() (string, error){
		// Appx path
		locateStoreDllAppx,

		// storeapi build path
		func() (string, error) { return locateStoreDllRepo(`msix\storeapi\x64\Debug\`) },
		func() (string, error) { return locateStoreDllRepo(`msix\storeapi\x64\Release\`) },

		// Project build path
		func() (string, error) { return locateStoreDllRepo(`msix\x64\Debug\storeapi\`) },
		func() (string, error) { return locateStoreDllRepo(`msix\x64\Release\storeapi\`) },
	}

	var accErr error
	for _, locate := range locators {
		path, err := locate()
		if err != nil {
			accErr = errors.Join(accErr, err)
			continue
		}

		return path, nil
	}

	return "", fmt.Errorf("could not locate Microsoft Store DLL: %v", accErr)
}

// locateStoreDll for the packaged application. The working dir is the
// root of the InstallLocation.
func locateStoreDllAppx() (path string, err error) {
	const appxPath = "./agent/storeapi.dll"

	// Appx: working directory is the Appx root
	if s, err := os.Stat(appxPath); err != nil {
		return "", err
	} else if s.IsDir() {
		return "", fmt.Errorf("%q: is a directory", appxPath)
	}

	return appxPath, nil
}

// locateStoreDll when running tests. Tests are run at the path of the testfile, so we know that
// the repo root is above the CWD.
func locateStoreDllRepo(path string) (string, error) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		return "", fmt.Errorf("could not find repository root: %v", err)
	}

	repoPath := filepath.Join(path, "storeapi.dll")

	candidate := filepath.Join(repoRoot, repoPath)
	if s, err := os.Stat(candidate); err != nil {
		return "", err
	} else if s.IsDir() {
		return "", fmt.Errorf("%q: is a directory", candidate)
	}

	return candidate, nil
}

func findRepositoryRoot() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", errors.New("could not get current working directory")
	}

	for {
		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("could not find repo root")
		}
		path = parent

		if s, err := os.Stat(filepath.Join(path, "go.work")); err == nil && !s.IsDir() {
			return parent, nil
		}
	}
}
