// Package microsoftstore is a stump to allow the project to build on Linux.
package microsoftstore

import (
	"errors"
	"unsafe"
)

type callResult struct {
	r1, r2 uintptr
	err    error
}

// LazyProc is a stub for the Windows version of LazyProc, a lazily loaded DLL function.
type LazyProc struct {
	cleanup    func(unsafe.Pointer)
	FindError  error
	CallResult callResult
}

func init() {
	singleton = &StoreAPIDLL{
		dll: LazyDLL{},
		generateUserJWT: LazyProc{cleanup: func(ptr unsafe.Pointer) {}, CallResult: callResult{
			err: ErrUnimplemented},
		},
		getSubscriptionExpirationDate: LazyProc{cleanup: func(ptr unsafe.Pointer) {}, CallResult: callResult{
			err: ErrUnimplemented},
		},
	}
}

// ErrUnimplemented is the error returned by all functions in this package by default on Linux.
var ErrUnimplemented = errors.New("this function is not implemented on Linux")

// LazyDLL is a stub for the Windows version of LazyDLL, a lazily loaded DLL.
type LazyDLL struct {
	Name      string
	LoadError error
}

// Load is a stub for the Windows version of Load, which loads the DLL and returns an error if it fails.
func (dll *LazyDLL) Load() error {
	return dll.LoadError
}

// Find is a stub for the Windows version of Find, which finds the specified procedure in the DLL and returns an error if it fails.
func (proc *LazyProc) Find() error {
	return proc.FindError
}

// Call is a stub for the Windows version of Call, which calls the specified procedure with the given arguments and returns the result and an error if it fails.
func (proc *LazyProc) Call(args ...uintptr) (r1, r2 uintptr, err error) {
	return proc.CallResult.r1, proc.CallResult.r2, proc.CallResult.err
}
