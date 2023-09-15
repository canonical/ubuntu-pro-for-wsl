#pragma once
#include <pplawait.h>

#include "base/DefaultContext.hpp"
#include "base/Exception.hpp"
#include "base/StoreService.hpp"

#include <chrono>
#include <cstdint>
#include <limits>

namespace StoreApi {

// Models the interesting user information the application can correlate
// when talking to external business servers about the subscription.
struct UserInfo {
  // The user ID that should be tracked in the Contract Server.
  std::string id;
};

// Adds functionality on top of the [StoreService] interesting to background
// server applications.
template <typename ContextType = DefaultContext>
class ServerStoreService : public StoreService<ContextType> {
 public:
  // Generates the user ID key (a.k.a the JWT) provided the server AAD [token]
  // and the [user] info whose ID the caller wants to have encoded in the JWT.
  std::string GenerateUserJwt(std::string token, UserInfo user) const {
    if (user.id.empty()) {
      throw Exception(StoreApi::ErrorCode::NoLocalUser);
    }

    auto jwt = this->context.GenerateUserJwt(token, user.id);
    if (jwt.empty()) {
      throw Exception(ErrorCode::EmptyJwt,
                      std::format("access token: {}", token));
    }

    return jwt;
  }

  // Returns the expiration time as the number of seconds since Unix epoch of
  // the current billing period if the current user is subscribed to this
  // product or the lowest int64_t otherwise (a date too far in the past). This
  // raw return value suits well for crossing ABI boundaries, such as returning
  // to a caller in Go.
  std::int64_t CurrentExpirationDate(std::string productId) {
    auto product = this->GetSubscriptionProduct(productId);
    if (!product.IsInUserCollection()) {
      return std::numeric_limits<std::int64_t>::lowest();
    }
    // C++20 guarantees that std::chrono::system_clock measures UNIX time.
    const auto dur = product.CurrentExpirationDate().time_since_epoch();

    // just need to convert the duration to seconds.
    return duration_cast<std::chrono::seconds>(dur).count();
  }

  // A factory returning the current user's [UserInfo].
  UserInfo CurrentUserInfo() const {
    auto hashes = this->context.AllLocallyAuthenticatedUserHashes();

    auto howManyUsers = hashes.size();
    if (howManyUsers < 1) {
      throw Exception(ErrorCode::NoLocalUser);
    }

    if (howManyUsers > 1) {
      throw Exception(ErrorCode::TooManyLocalUsers,
                      std::format("Expected one but found {}", howManyUsers));
    }

    return UserInfo{.id = hashes[0]};
  }
};

}  // namespace StoreApi
