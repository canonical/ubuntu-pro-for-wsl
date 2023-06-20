// Exports two functions exposing information about a product subscription from
// MS Store that the background agent is interested in. They both follow a
// simple protocol of returning negative int64_t values for errors that the
// caller can translate to the most suitable error system used in their side of
// the ABI. Positive values are the actual response, which may or may not
// combine with further output parameters. Zero should never be returned.
#pragma once

#include <agent/ServerStoreService.hpp>

#define DLL_EXPORT __declspec(dllexport)

extern "C" {
// Returns a positive integer representing the UNIX time of the current user
// subscription expiration date.
DLL_EXPORT int64_t GetSubscriptionExpirationDate(const char* productID,
                                                 int32_t length);

// Outputs the user JWT via the [jwtBuf] output parameter and returns the buffer
// length. The caller is responsible for freeing the memory region pointed by
// [jwtBuf] by calling CoTaskMemFree.
DLL_EXPORT int64_t GenerateUserJWT(const char* accessToken,
                                   int32_t accessTokenLen,
                                   // output
                                   char* jwtBuf);
}

// Document error constants so we can translate those as Go errors.
enum class Errors : int64_t {
  NotSubscribed = std::numeric_limits<int64_t>::lowest(),
  AllocationFailure = -10,
  // input string argument errors
  Null = -9,
  NegativeLength = -8,
  TooBigLength = -7,
  ZeroLength = -6,
  // runtime errors (aka exceptions)
  StoreAPI = -3,
  WinRT = -2,
  Unknown = -1,
  // Not an error.
  None = 0,
};
