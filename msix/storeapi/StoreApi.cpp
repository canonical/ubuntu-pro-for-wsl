#include "StoreApi.hpp"

#include <combaseapi.h>
#include <winrt/base.h>

#include <agent/ServerStoreService.hpp>
#include <base/Exception.hpp>
#include <cstring>  // For strnlen
#include <exception>
#include <string>

#include "framework.hpp"

#ifndef DNDEBUG
#include <format>
#include <iostream>
#endif

// Syntactic sugar to convert the enum [value] into a Int.
constexpr Int toInt(StoreApi::ErrorCode value) {
  return static_cast<Int>(value);
}

// The maximum token length expected + 1 (the null terminator).
static constexpr std::size_t MaxTokenLen = 4097;

// The maximum product ID string length expected as an input + 1 (the null
// terminator). In practice it's much smaller. This reserves room for the
// future.
static constexpr std::size_t MaxProductIdLen = 129;

// Makes sure [input] is not a nullptr and it's a null-terminated string with
// length smaller than [maxLength].
StoreApi::ErrorCode validateArg(const char* input, std::size_t maxLength);

void logError(std::string_view functionName, std::string_view errMsg) {
#ifndef DNDEBUG
  std::cerr << std::format("storeapi: {}: {}\n", functionName, errMsg);
#endif
}

#define LOG_ERROR(msg)           \
  do {                           \
    logError(__FUNCTION__, msg); \
  } while (0)

Int GetSubscriptionExpirationDate(const char* productID,
                                  std::int64_t* expirationUnix) {
  if (auto err = validateArg(productID, MaxProductIdLen);
      err != StoreApi::ErrorCode::None) {
    return toInt(err);
  }

  if (expirationUnix == nullptr) {
    return toInt(StoreApi::ErrorCode::NullOutputPtr);
  }

  try {
    StoreApi::ServerStoreService service{};

    *expirationUnix = service.CurrentExpirationDate(productID);
    return 0;

  } catch (const StoreApi::Exception& err) {
    LOG_ERROR(err.what());
    return toInt(err.code());
  } catch (const winrt::hresult_error& err) {
    LOG_ERROR(winrt::to_string(err.message()));
    return toInt(StoreApi::ErrorCode::WinRT);
  } catch (const std::exception& err) {
    LOG_ERROR(err.what());
    return toInt(StoreApi::ErrorCode::Unknown);
  }
}

Int GenerateUserJWT(const char* accessToken, char** userJWT,
                    std::uint64_t* userJWTLen) {
  if (auto err = validateArg(accessToken, MaxTokenLen);
      err != StoreApi::ErrorCode::None) {
    return toInt(err);
  }

  if (userJWT == nullptr || userJWTLen == nullptr) {
    return toInt(StoreApi::ErrorCode::NullOutputPtr);
  }

  try {
    StoreApi::ServerStoreService service{};
    auto user = service.CurrentUserInfo();
    const std::string jwt = service.GenerateUserJwt(accessToken, user);

    // Allocates memory using some OS API so we can free this buffer on the
    // other side of the ABI without assumptions on specifics of the programming
    // language runtime in their side.
    const auto length = jwt.size();
    auto* buffer = static_cast<char*>(::CoTaskMemAlloc(length));
    if (buffer == nullptr) {
      return toInt(StoreApi::ErrorCode::AllocationFailure);
    }

    std::memcpy(buffer, jwt.c_str(), length);
    *userJWT = buffer;
    *userJWTLen = length;
    return 0;

  } catch (const StoreApi::Exception& err) {
    LOG_ERROR(err.what());
    return toInt(err.code());
  } catch (const winrt::hresult_error& err) {
    LOG_ERROR(winrt::to_string(err.message()));
    return toInt(StoreApi::ErrorCode::WinRT);
  } catch (const std::exception& err) {
    LOG_ERROR(err.what());
    return toInt(StoreApi::ErrorCode::Unknown);
  }
}

StoreApi::ErrorCode validateArg(const char* input, std::size_t maxLength) {
  if (input == nullptr) {
    return StoreApi::ErrorCode::NullInputPtr;
  }

  // since the null terminator is not counted by strnlen, if maxLength is
  // returned, that means the string is longer than maxLenght.
  const auto length = strnlen(input, maxLength);

  if (length == 0) {
    return StoreApi::ErrorCode::ZeroLength;
  }

  if (length == maxLength) {
    return StoreApi::ErrorCode::TooBigLength;
  }

  return StoreApi::ErrorCode::None;
}
