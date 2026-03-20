package microsoftstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/decorate"
)

// StoreAPIDLL is a struct that holds the LazyDLL and LazyProcs for the StoreAPI.dll, as well as a mutex to protect loading the DLL.
type StoreAPIDLL struct {
	dllMu                                          sync.Mutex
	dll                                            LazyDLL
	generateUserJWT, getSubscriptionExpirationDate LazyProc
}

var singleton *StoreAPIDLL

func init() {
	reset()
}

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
	if _, err = singleton.call(
		&singleton.generateUserJWT,
		uintptr(unsafe.Pointer(accessToken)),
		uintptr(unsafe.Pointer(&userJWTbegin)),
		uintptr(unsafe.Pointer(&userJWTlen)),
	); err != nil {
		return "", fmt.Errorf("GenerateUserJWT: %w", err)
	}

	//nolint:gosec // This is the way of freeing userJWTbegin per storeapi's API definition
	// defer windows.CoTaskMemFree(unsafe.Pointer(userJWTbegin))
	defer singleton.generateUserJWT.cleanup(unsafe.Pointer(userJWTbegin))

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
	if _, err = singleton.call(
		&singleton.getSubscriptionExpirationDate,
		uintptr(unsafe.Pointer(prodID)),
		uintptr(unsafe.Pointer(&expDate)),
	); err != nil {
		return time.Time{}, err
	}

	return time.Unix(expDate, 0), nil
}

// call forces the proc and DLL to load before calling it.
// Use this instead of proc.Call to avoid panics.
//
//nolint:unparam // Return value is provided to follow convention.
func (d *StoreAPIDLL) call(proc *LazyProc, args ...uintptr) (int64, error) {
	if err := d.loadDll(); err != nil {
		return 0, err
	}

	// Avoid panic in Call by calling Find beforehand.
	if err := proc.Find(); err != nil {
		return 0, err
	}

	hresult, _, err := proc.Call(args...)
	//nolint:gosec //G115 it's OK because we want the wraparound behaviour. Although the API
	//returns a uintptr, the value is actually a signed int64 and can be negative, thus
	//guaranteed to fit in an int64.
	return checkError(int64(hresult), err)
}

// loadDll finds the dll and ensures it loads.
func (d *StoreAPIDLL) loadDll() error {
	d.dllMu.Lock()
	defer d.dllMu.Unlock()

	if d.dll.Name != "" {
		return nil
	}

	path, err := locateStoreDll()
	if err != nil {
		return errors.Join(ErrCantLoadDLL, err)
	}

	d.dll.Name = path
	if err = d.dll.Load(); err != nil {
		d.dll.Name = ""
		return errors.Join(ErrCantLoadDLL, err)
	}

	return nil
}

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

// checkError inspects the values of hres and err to determine what kind of error we have, if any, according to the rules of syscall/dll_windows.go.
func checkError(hres int64, err error) (int64, error) {
	// From syscall/dll_windows.go (*Proc).Call doc:
	// > Callers must inspect the primary return value to decide whether an
	//   error occurred [...] before consulting the error.
	// There is no possibility of nil  error, the `err` return value is always constructed with the
	// result of `GetLastError()` which could have been set by something completely
	// unrelated to our code some time in the past, as well as it could be `ERROR_SUCCESS` which is the `Errno(0)`.
	// If the act of calling the API fails (not the function we're calling, but the attempt to call it), then we'd
	// have a meaningful `syscall.Errno` object via the `err` parameter, related to the actual failure (like a function not found in this DLL)
	// Since our implementation of the store API doesn't touch errno the call should return `hres`
	// in our predefined range plus garbage in the `err` argument, thus we only care about the `hres` in this case.
	if e := NewStoreAPIError(hres); e != nil {
		return hres, fmt.Errorf("storeApi returned error code %d: %w", hres, e)
	}

	// Supposedly unreachable: proc.Call must always return a non-nil syscall.Errno
	if err == nil {
		return hres, nil
	}

	var target syscall.Errno
	if b := errors.As(err, &target); !b {
		// Supposedly unreachable: proc.Call must always return a non-nil syscall.Errno
		return hres, err
	}

	// The act of calling our API didn't succeed, function not found in the DLL for example:
	if target != syscall.Errno(0) {
		return hres, fmt.Errorf("failed syscall to storeApi: %v (syscall errno %d)", target, err)
	}

	// A non-error value in hres plus ERROR_SUCCESS in err.
	// This shouldn't happen in the current store API implementation anyway.
	return hres, nil
}
