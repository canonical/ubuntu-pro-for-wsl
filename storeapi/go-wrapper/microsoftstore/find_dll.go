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

		// CMake build paths (standalone build from storeapi/dll)
		func() (string, error) { return locateStoreDllRepo(`build\dll\Debug\`) },
		func() (string, error) { return locateStoreDllRepo(`build\dll\Release\`) },

		// CMake build paths (root build)
		func() (string, error) { return locateStoreDllRepo(`build\x64\storeapi\dll\Debug\`) },
		func() (string, error) { return locateStoreDllRepo(`build\x64\storeapi\dll\Release\`) },
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
	exec, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not find path of executable: %v", err)
	}

	// DLL is located in the same directory as the agent executable.
	dllPath := filepath.Join(
		filepath.Dir(exec),
		"storeapi.dll",
	)

	if s, err := os.Stat(dllPath); err != nil {
		return "", err
	} else if s.IsDir() {
		return "", fmt.Errorf("%q: is a directory", dllPath)
	}

	return dllPath, nil
}

// locateStoreDll when running tests. Tests are run at the path of the testfile, so we know that
// the repo root is above the CWD.
func locateStoreDllRepo(path string) (string, error) {
	repoRoot, err := findWorkspaceRoot()
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
