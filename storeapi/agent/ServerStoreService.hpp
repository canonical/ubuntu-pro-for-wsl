#pragma once
#include <ctime>

#include "base/Context.hpp"
#include "base/Exception.hpp"
#include "base/StoreService.hpp"

namespace StoreApi {

// Models the interesting user information the application can correlate
// when talking to external business servers about the subscription.
struct UserInfo {
  // The user ID that should be tracked in the Contract Server.
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
    if (user.id.empty()) {
      throw Exception(StoreApi::ErrorCode::NoLocalUser);
    }

    auto hToken = winrt::to_hstring(token);
    auto jwt = co_await this->context.GenerateUserJwt(hToken, user.id);
    if (jwt.empty()) {
      throw Exception(ErrorCode::EmptyJwt,
                      std::format("access token: {}", token));
    }

    co_return winrt::to_string(jwt);
  }

  // Returns the expiration time as the number of seconds since Unix epoch of
  // the current billing period if the current user is subscribed to this
  // product or the lowest int64_t otherwise (a date too far in the past). This
  // raw return value suits well for crossing ABI boundaries, such as returning
  // to a caller in Go.
  concurrency::task<std::int64_t> CurrentExpirationDate(std::string productId) {
    auto product = co_await this->GetSubscriptionProduct(productId);
    if (!product.IsInUserCollection()) {
      co_return std::numeric_limits<std::int64_t>::lowest();
    }
    // C++20 guarantees that std::chrono::system_clock measures UNIX time.
    const auto t = winrt::clock::to_sys(product.CurrentExpirationDate());
    const auto dur = t.time_since_epoch();

    // just need to convert the duration to seconds.
    co_return duration_cast<std::chrono::seconds>(dur).count();
  }
};

}  // namespace StoreApi
