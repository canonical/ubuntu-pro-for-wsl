// Exports functions exposing information about a product subscription from
// MS Store that the background agent is interested in. They both follow a
// simple protocol of returning negative integer values for errors that the
// caller can translate to the most suitable error system used in their side of
// the ABI. Zero or positive values have no special meaning other than success.
#pragma once

#include <agent/ServerStoreService.hpp>
#include <cstdint>

extern "C" {
// Go will call us with uintptr's, which are unsigned and large enough to hold
// any pointer. The equivalent for that is C99 uintptr_t (also C++11 and
// forward). On pointers we are safe, but accepting uintptrs into a int32_t is a
// narrowing conversion on x64 platforms. On those platforms we could rely on
// uintptr_t being the same as uint64_t. To be more generic and future proof we
// typedef from intptr_t (the signed version that can still hold any pointer),
// so we can preserve the signed nature of actual integer (non-pointer) values.
using Int = std::intptr_t;

#define DLL_EXPORT __declspec(dllexport)

// Returns via the [expirationUnix] output parameter a positive integer
// representing the expiration date as UNIX time of current user's subscription
// to the product represented by the null-terminated string [productID].
DLL_EXPORT Int GetSubscriptionExpirationDate(const char* productID,
                                             // output
                                             Int* expirationUnix);

// Outputs the user JWT string via the [userJWT] output parameter and its
// length via [userJWTLen], allowing the server identified via the [accessToken]
// to query information about the current user's subscriptions in behalf of our
// app. The [accessToken] is required to be a null-terminated string. The caller
// is responsible for freeing the memory region pointed by [userJWT] by calling
// CoTaskMemFree.
DLL_EXPORT Int GenerateUserJWT(const char* accessToken,
                               // output
                               char** userJWT, Int* userJWTLen);

// Document error constants so we can translate those as Go errors.
enum class Errors : Int {
  NotSubscribed = std::numeric_limits<Int>::lowest(),
  AllocationFailure = -10,
  // input string argument errors
  NullInputPtr = -9,
  TooBigLength = -8,
  ZeroLength = -7,
  // output parameter errors
  NullOutputPtr = -6,
  // runtime errors (aka exceptions)
  StoreAPI = -3,
  WinRT = -2,
  Unknown = -1,
  // Not an error.
  None = 0,
};
}
