// Package microsoftstore is a wrapper around storeapi.dll
package microsoftstore

import "errors"

// StoreAPIError are the error constants in the store api.
type StoreAPIError int64

// Keep up-to-date with `storeapi\base\Exception.hpp`.
const (
	ErrNotSubscribed StoreAPIError = iota - 128
	ErrNoProductsFound
	ErrTooManyProductsFound
	ErrInvalidUserInfo
	ErrNoLocalUser
	ErrTooManyLocalUsers
	ErrEmptyJwt

	ErrAllocationFailure StoreAPIError = -10
	ErrNullInputPtr      StoreAPIError = -9
	ErrTooBigLength      StoreAPIError = -8
	ErrZeroLength        StoreAPIError = -7
	ErrNullOutputPtr     StoreAPIError = -6
	ErrStoreAPI          StoreAPIError = -3
	ErrWinRT             StoreAPIError = -2
	ErrUnknown           StoreAPIError = -1
	ErrSuccess           StoreAPIError = 0
)

// NewStoreAPIError creates StoreAPIError from the result of a call to the storeAPI DLL.
func NewStoreAPIError(hresult int64) error {
	if err := StoreAPIError(hresult); err < ErrSuccess {
		return err
	}
	return nil
}

func (err StoreAPIError) Error() string {
	switch err {
	case ErrNotSubscribed:
		return "current user not subscribed to this product"
	case ErrNoProductsFound:
		return "query found no products"
	case ErrTooManyProductsFound:
		return "query found too many products"
	case ErrInvalidUserInfo:
		return "no locally authenticated user could be found"
	case ErrNoLocalUser:
		return "invalid user info. Maybe not a real user session"
	case ErrTooManyLocalUsers:
		return "too many locally authenticated users"
	case ErrEmptyJwt:
		return "empty user JWT was generated"
	case ErrAllocationFailure:
		return "allocation failure"
	case ErrTooBigLength:
		return "length too large"
	case ErrZeroLength:
		return "length cannot be zero"
	case ErrNullOutputPtr:
		return "output buffer cannot be null"
	case ErrStoreAPI:
		return "error at the store API"
	case ErrWinRT:
		return "error at the Windows Runtime"
	case ErrUnknown:
		return "unexpected error"
	case ErrSuccess:
		return "success"
	default:
		return "undefined"
	}
}

// ErrCantLoadDLL is the error returned when the DLL cannot be loaded, which can happen if the DLL is not present or if there are missing dependencies.
var ErrCantLoadDLL = errors.New("storeapi.dll not found or failed to load")

// ErrUnimplemented is the error returned by all functions in this package by default on Linux.
var ErrUnimplemented = errors.New("function is not implemented for this platform")
