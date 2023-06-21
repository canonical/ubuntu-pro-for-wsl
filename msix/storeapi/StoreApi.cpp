#include "StoreApi.hpp"

#include "framework.hpp"

// Syntactic sugar to convert the enum [value] into a Int.
constexpr Int toInt(Errors value) { return static_cast<Int>(value); }

// The maximum token length expected.
static constexpr Int MaxTokenLen = 4096;

// The maximum product ID string length expected as an input. In practice the
// size is much lower. This reserves room for the future.
static constexpr Int MaxProductIdLen = 128;

// Groups together a char pointer and a length to prevent easy swappable
// parameters in function calls.
struct RawString {
  const char* data;
  Int length;
};

// Sanity checks the [input] string argument against some non-sensical mistakes
// (such as negative length) and against a [maxLength] allowed by the caller.
Errors validateArg(RawString input, Int maxLength);

Int GetSubscriptionExpirationDate(const char* productID, Int length,
                                  Int* expirationUnix) {
  if (auto err =
          validateArg({.data = productID, .length = length}, MaxProductIdLen);
      err != Errors::None) {
    return toInt(err);
  }

  if (expirationUnix == nullptr) {
    return toInt(Errors::NullOutputPtr);
  }

  try {
    StoreApi::ServerStoreService service{};

    const std::time_t expiration =
        service
            .CurrentExpirationDate(
                {productID, static_cast<std::size_t>(length)})
            .get();

    *expirationUnix = static_cast<Int>(expiration);
    return 0;
  } catch (const StoreApi::Exception&) {
    return toInt(Errors::StoreAPI);
  } catch (const winrt::hresult_error&) {
    return toInt(Errors::WinRT);
  } catch (const std::exception&) {
    return toInt(Errors::Unknown);
  }
}

Int GenerateUserJWT(const char* accessToken, Int accessTokenLen, char** userJWT,
                    Int* userJWTLen) {
  if (auto err = validateArg({.data = accessToken, .length = accessTokenLen},
                             MaxTokenLen);
      err != Errors::None) {
    return toInt(err);
  }

  if (userJWT == nullptr || userJWTLen == nullptr) {
    return toInt(Errors::NullOutputPtr);
  }

  try {
    auto user = StoreApi::UserInfo::Current().get();
    std::string serverToken{accessToken,
                            static_cast<std::size_t>(accessTokenLen)};

    StoreApi::ServerStoreService service{};
    const std::string jwt = service.GenerateUserJwt(serverToken, user).get();

    // Allocates memory using some OS API so we can free this buffer on the
    // other side of the ABI without assumptions on specifics of the programming
    // language runtime in their side.
    const Int length = jwt.size();
    auto* buffer = static_cast<char*>(::CoTaskMemAlloc(length));
    if (buffer == nullptr) {
      return toInt(Errors::AllocationFailure);
    }

    std::memcpy(buffer, jwt.c_str(), length);
    *userJWT = buffer;
    *userJWTLen = length;
    return 0;

  } catch (const StoreApi::Exception&) {
    return toInt(Errors::StoreAPI);
  } catch (const winrt::hresult_error&) {
    return toInt(Errors::WinRT);
  } catch (const std::exception&) {
    return toInt(Errors::Unknown);
  }
}

Errors validateArg(RawString input, Int maxLength) {
  if (input.data == nullptr) {
    return Errors::NullInputPtr;
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
