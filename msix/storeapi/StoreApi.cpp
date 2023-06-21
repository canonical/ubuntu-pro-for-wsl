#include "StoreApi.hpp"

#include "framework.hpp"

// Syntactic sugar to convert the enum [value] into a int64_t.
constexpr int64_t toInt64(Errors value) { return static_cast<int64_t>(value); }

// The maximum token length expected.
static constexpr int32_t MaxTokenLen = 4096;

// The maximum product ID string length expected as an input. In practice the
// size is much lower. This reserves room for the future.
static constexpr int32_t MaxProductIdLen = 128;

// Groups together a char pointer and a length to prevent easy swappable
// parameters in function calls.
struct RawString {
  const char* data;
  int32_t length;
};

// Sanity checks the [input] string argument against some non-sensical mistakes
// (such as negative length) and against a [maxLength] allowed by the caller.
Errors validateArg(RawString input, int32_t maxLength);

int64_t GetSubscriptionExpirationDate(const char* productID, int32_t length) {
  if (auto err =
          validateArg({.data = productID, .length = length}, MaxProductIdLen);
      err != Errors::None) {
    return toInt64(err);
  }

  try {
    StoreApi::ServerStoreService service{};

    const std::time_t expiration =
        service
            .CurrentExpirationDate({productID, static_cast<uint64_t>(length)})
            .get();

    return expiration;
  } catch (const StoreApi::Exception&) {
    return toInt64(Errors::StoreAPI);
  } catch (const winrt::hresult_error&) {
    return toInt64(Errors::WinRT);
  } catch (const std::exception&) {
    return toInt64(Errors::Unknown);
  }
}

int64_t GenerateUserJWT(const char* accessToken, int32_t accessTokenLen,
                        char** jwtBuf) {
  if (auto err = validateArg({.data = accessToken, .length = accessTokenLen},
                             MaxTokenLen);
      err != Errors::None) {
    return toInt64(err);
  }

  try {
    auto user = StoreApi::UserInfo::Current().get();
    std::string serverToken{accessToken, static_cast<uint64_t>(accessTokenLen)};

    StoreApi::ServerStoreService service{};
    const std::string jwt = service.GenerateUserJwt(serverToken, user).get();

    // Allocates memory using some OS API so we can free this buffer on the
    // other side of the ABI without assumptions on specifics of the programming
    // language runtime in their side.
    const int64_t length = jwt.size();
    auto* buffer = static_cast<char*>(::CoTaskMemAlloc(length));
    if (buffer == nullptr) {
      return toInt64(Errors::AllocationFailure);
    }

    std::memcpy(buffer, jwt.c_str(), length);
    *jwtBuf = buffer;
    return length;

  } catch (const StoreApi::Exception&) {
    return toInt64(Errors::StoreAPI);
  } catch (const winrt::hresult_error&) {
    return toInt64(Errors::WinRT);
  } catch (const std::exception&) {
    return toInt64(Errors::Unknown);
  }
}

Errors validateArg(RawString input, int32_t maxLength) {
  if (input.data == nullptr) {
    return Errors::Null;
  }

  if (input.length < 0) {
    return Errors::NegativeLength;
  }

  if (input.length == 0) {
    return Errors::ZeroLength;
  }

  if (input.length > maxLength) {
    return Errors::TooBigLength;
  }

  return Errors::None;
}
