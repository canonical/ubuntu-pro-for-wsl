// Package microsoftstore intrerfaces with the storeapi.dll library.
package microsoftstore

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/decorate"
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
func GenerateUserJWT(azureADToken string) (jwt string, err error) {
	defer decorate.OnError(&err, "couldn't generate a user JWT from the Microsoft Store API")

	accessToken, err := syscall.BytePtrFromString(azureADToken)
	if err != nil {
		return "", fmt.Errorf("could not convert the auth token to a byte array: %v", err)
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
		return "", fmt.Errorf("GenerateUserJWT: %w", err)
	}

	//nolint:gosec // This is the way of freeing userJWTbegin per storeapi's API definition
	defer windows.CoTaskMemFree(unsafe.Pointer(userJWTbegin))

	//nolint:gosec // This is the way of converting a Win32 string to a Go string
	return string(unsafe.Slice(userJWTbegin, userJWTlen)), nil
}

// GetSubscriptionExpirationDate returns the expiration date for the current subscription.
func GetSubscriptionExpirationDate() (tm time.Time, err error) {
	defer decorate.OnError(&err, "could not get the Ubuntu Pro subscription expiration date from the Microsoft Store API")

	prodID, err := syscall.BytePtrFromString(common.MsStoreProductID)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not convert the productID to a byte array: %v", err)
	}

	var expDate int64

	//nolint:gosec // No other way of calling a Dll proc
	if _, err = call(
		getSubscriptionExpirationDate,
		uintptr(unsafe.Pointer(prodID)),
		uintptr(unsafe.Pointer(&expDate)),
	); err != nil {
		return time.Time{}, err
	}

	return time.Unix(expDate, 0), nil
}

// call forces the proc and DLL to load before calling it, and cleans up the output.
// Use this instead of proc.Call to avoid panics.
//
//nolint:unparam // Return value is provided to follow convention.
func call(proc *syscall.LazyProc, args ...uintptr) (int64, error) {
	if err := loadDll(); err != nil {
		return 0, err
	}

	// Avoid panic in Call by calling Find beforehand.
	if err := proc.Find(); err != nil {
		return 0, err
	}

	hresult, _, err := proc.Call(args...)
	return checkError(int64(hresult), err)
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
		return fmt.Errorf("could not find Microsoft Store API dll: %v", err)
	}

	dll.Name = path
	if err = dll.Load(); err != nil {
		dll.Name = ""
		return fmt.Errorf("could not load the Microsoft Store API dll: %v", err)
	}

	return nil
}
