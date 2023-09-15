#include <gtest/gtest.h>

#include <agent/ServerStoreService.hpp>

#include "stubs.hpp"

namespace StoreApi {

using namespace ::testing;

TEST(UserInfo, PredictableSizes) {
  auto user = UserInfo::Current().get();
  auto size = user.id.size();
  EXPECT_TRUE(size == 0 || size == 64)
      << "User ID of unexpected size: " << size << " <"
      << winrt::to_string(user.id) << '\n';
}

TEST(ServerStoreService, EmptyJwtThrows) {
  auto service = ServerStoreService<EmptyJwtContext>{};
  UserInfo user{.id = L"my@name.com"};
  EXPECT_THROW(
      {
        auto jwt = service.GenerateUserJwt("this-is-a-web-token", user).get();
      },
      Exception);
}

TEST(ServerStoreService, NonEmptyJwtNeverThrows) {
  auto service = ServerStoreService<IdentityJwtContext>{};
  UserInfo user{.id = L"my@name.com"};
  std::string token{"this-is-a-web-token"};
  auto jwt = service.GenerateUserJwt(token, user).get();
  EXPECT_EQ(token, jwt);
}

TEST(ServerStoreService, RealServerFailsUnderTest) {
  auto service = ServerStoreService{};
  UserInfo user{.id = L"my@name.com"};
  std::string token{"this-is-a-web-token"};
  // This fails because the test is not an app deployed through the store.
  EXPECT_THROW({ auto jwt = service.GenerateUserJwt(token, user).get(); },
               Exception);
}

TEST(ServerStoreService, ExpirationDateUnsubscribed) {
  auto service = ServerStoreService<NeverSubscribedContext>{};

  auto expiration = service.CurrentExpirationDate("my-awesome-addon");

  EXPECT_EQ(std::numeric_limits<std::int64_t>::lowest(), expiration);
}

TEST(ServerStoreService, ExpirationDateEpoch) {
  auto service = ServerStoreService<UnixEpochContext>{};

  std::tm tm = {
      .tm_sec = 0,
      .tm_min = 0,
      .tm_hour = 0,
      .tm_mday = 1,
      .tm_mon = 0,    // 1 - 1,
      .tm_year = 70,  // 1970 - 1900,
      .tm_wday = 0,
      .tm_yday = 0,
      .tm_isdst = -1,  // Use DST value from local time zone
  };
  auto unix_epoch = static_cast<int64_t>(timegm(&tm));

  auto expiration = service.CurrentExpirationDate("my-awesome-addon");

  EXPECT_EQ(unix_epoch, expiration);
}

}  // namespace StoreApi
