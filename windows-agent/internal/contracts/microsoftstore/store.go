package microsoftstore

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// Loading must be delayed for tests: the path to the DLL is known only relative to this file.
	// At module load-time, the working directory can be anywhere.
	// During the tests, the working directory is at a known location.
	dll   = syscall.NewLazyDLL("")
	dllMu sync.Mutex

	generateUserJWT               = dll.NewProc("GenerateUserJWT")
	getSubscriptionExpirationDate = dll.NewProc("GetSubscriptionExpirationDate")
)

// GenerateUserJWT takes an azure AD server access token and returns a Windows store token.
func GenerateUserJWT(azureADToken string) (string, error) {
	accessToken, err := syscall.BytePtrFromString(azureADToken)
	if err != nil {
		return "", errors.New("could not convert the AzureAD token to a byte array")
	}

	var userJWTbegin *byte
	var userJWTlen uint64

	//nolint:gosec // No other way of calling a Dll proc
	if _, err = call(
		generateUserJWT,
		uintptr(unsafe.Pointer(accessToken)),
		uintptr(unsafe.Pointer(&userJWTbegin)),
		uintptr(unsafe.Pointer(&userJWTlen)),
	); err != nil {
		return "", err
	}

	//nolint:gosec // This is the way of freeing userJWTbegin per storeapi's API definition
	defer windows.CoTaskMemFree(unsafe.Pointer(userJWTbegin))

	return string(unsafe.Slice(userJWTbegin, userJWTlen)), nil
}

// call forces the proc and DLL to load before calling it, and cleans up the output.
// Use this instead of proc.Call to avoid panics.
//
//nolint:unparam // Return value is provided to follow convention.
func call(proc *syscall.LazyProc, args ...uintptr) (int, error) {
	if err := loadDll(); err != nil {
		return 0, err
	}

	// Avoid panic in Call by calling Find beforehand.
	if err := proc.Find(); err != nil {
		return 0, err
	}

	hresult, _, err := proc.Call(args...)
	if err != nil && !errors.Is(err, syscall.Errno(0)) {
		return hresult, fmt.Errorf("%s: %v", proc.Name, err)
	}

	if err := NewStoreAPIError(hresult); err != nil {
		return hresult, fmt.Errorf("%s returned error code %d: %w", generateUserJWT.Name, hresult, err)
	}

	return hresult, nil
}

// loadDll finds the dll and ensures it loads.
func loadDll() error {
	dllMu.Lock()
	defer dllMu.Unlock()

	if dll.Name != "" {
		return nil
	}

	path, err := locateStoreDll()
	if err != nil {
		return fmt.Errorf("could not find Windows Store API dll: %v", err)
	}

	dll.Name = path
	if err = dll.Load(); err != nil {
		dll.Name = ""
		return err
	}

	return nil
}
