// Package microsoftstore intrerfaces with the storeapi.dll library.
package microsoftstore

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// LazyProc is a wrapper around syscall.LazyProc that allows us to delay loading the DLL until it's needed.
type LazyProc struct {
	proc    *syscall.LazyProc
	cleanup func(unsafe.Pointer)
}

func (p *LazyProc) Find() error {
	return p.proc.Find()
}

func (p *LazyProc) Call(args ...uintptr) (r1, r2 uintptr, err error) {
	return p.proc.Call(args...)
}

// LazyDLL is a wrapper around syscall.LazyDLL that allows us to delay loading the DLL until it's needed.
// This is necessary because the path to the DLL is only known relative to this file, and at module load-time, the working directory can be anywhere.
// During the tests, the working directory is at a known location, so we can load the DLL then.
type LazyDLL struct {
	*syscall.LazyDLL
}

func init() {
	lazyDLL := LazyDLL{syscall.NewLazyDLL("")}
	singleton = &StoreAPIDLL{
		// Loading must be delayed for tests: the path to the DLL is known only relative to this file.
		// At module load-time, the working directory can be anywhere.
		// During the tests, the working directory is at a known location.
		dll:                           lazyDLL,
		generateUserJWT:               LazyProc{proc: lazyDLL.NewProc("GenerateUserJWT"), cleanup: func(ptr unsafe.Pointer) { windows.CoTaskMemFree(ptr) }},
		getSubscriptionExpirationDate: LazyProc{proc: lazyDLL.NewProc("GetSubscriptionExpirationDate"), cleanup: func(ptr unsafe.Pointer) { /* no cleanup needed for this proc */ }},
	}

}
