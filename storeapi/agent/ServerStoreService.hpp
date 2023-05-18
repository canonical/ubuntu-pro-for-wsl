#pragma once
#include <ctime>

#include "base/Context.hpp"
#include "base/Exception.hpp"
#include "base/StoreService.hpp"

namespace StoreApi {

// Models the interesting user information the application can correlate
// when talking to external business servers about the subscription.
struct UserInfo {
  // The user ID that must be tracked in the Contract Server.
  winrt::hstring id;

  // An asynchronous factory returning [UserInfo] of the current user.
  static concurrency::task<UserInfo> Current();
};

// Adds functionality on top of the [StoreService] interesting to background
// server applications.
template <typename ContextType = Context>
class ServerStoreService : public StoreService<ContextType> {
 public:
  // Generates the user ID key (a.k.a the JWT) provided the server AAD [token]
  // and the [user] info whose ID the caller wants to have encoded in the JWT.
  concurrency::task<std::string> GenerateUserJwt(std::string token,
                                                 UserInfo user) {
    auto hToken = winrt::to_hstring(token);
    auto jwt = co_await this->context.GenerateUserJwt(hToken, user.id);
    if (jwt.empty()) {
      throw Exception("Empty JWT was generated.");
    }

    co_return winrt::to_string(jwt);
  }

  // Returns the expiration date of the current billing period if the current
  // user is subscribed to this product or the lowest time_t otherwise (a date
  // too far in the past). This raw return value suits well for crossing ABI
  // boundaries, such as returning to a caller in Go.
  concurrency::task<std::time_t> CurrentExpirationDate(std::string productId) {
    auto product = co_await this->GetSubscriptionProduct(productId);
    if (!product.IsInUserCollection()) {
      co_return std::numeric_limits<std::time_t>::lowest();
    }

    co_return winrt::clock::to_time_t(product.CurrentExpirationDate());
  }
};

}  // namespace StoreApi
