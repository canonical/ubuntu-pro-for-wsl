#include "StoreApi.hpp"

#include "framework.hpp"

// Syntactic sugar to convert the enum [value] into a Int.
constexpr Int toInt(StoreApi::ErrorCode value) { return static_cast<Int>(value); }

// The maximum token length expected + 1 (the null terminator).
static constexpr std::size_t MaxTokenLen = 4097;

// The maximum product ID string length expected as an input + 1 (the null
// terminator). In practice it's much smaller. This reserves room for the
// future.
static constexpr std::size_t MaxProductIdLen = 129;

// Makes sure [input] is not a nullptr and it's a null-terminated string with
// length smaller than [maxLength].
StoreApi::ErrorCode validateArg(const char* input, std::size_t maxLength);

Int GetSubscriptionExpirationDate(const char* productID,
                                  std::int64_t* expirationUnix) {
  if (auto err = validateArg(productID, MaxProductIdLen); err != StoreApi::ErrorCode::None) {
    return toInt(err);
  }

  if (expirationUnix == nullptr) {
    return toInt(StoreApi::ErrorCode::NullOutputPtr);
  }

  try {
    StoreApi::ServerStoreService service{};

    *expirationUnix = service.CurrentExpirationDate(productID).get();
    return 0;

  } catch (const StoreApi::Exception& err) {
    return toInt(err.code());
  } catch (const winrt::hresult_error&) {
    return toInt(StoreApi::ErrorCode::WinRT);
  } catch (const std::exception&) {
    return toInt(StoreApi::ErrorCode::Unknown);
  }
}

Int GenerateUserJWT(const char* accessToken, char** userJWT,
                    std::uint64_t* userJWTLen) {
  if (auto err = validateArg(accessToken, MaxTokenLen); err != StoreApi::ErrorCode::None) {
    return toInt(err);
  }

  if (userJWT == nullptr || userJWTLen == nullptr) {
    return toInt(StoreApi::ErrorCode::NullOutputPtr);
  }

  try {
    auto user = StoreApi::UserInfo::Current().get();

    StoreApi::ServerStoreService service{};
    const std::string jwt = service.GenerateUserJwt(accessToken, user).get();

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
    return toInt(err.code());
  } catch (const winrt::hresult_error&) {
    return toInt(StoreApi::ErrorCode::WinRT);
  } catch (const std::exception&) {
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
