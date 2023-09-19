#include <gtest/gtest.h>

#include <agent/ServerStoreService.hpp>

#include "stubs.hpp"

namespace StoreApi {

using namespace ::testing;

TEST(ServerStoreService, NoUsersLikeInCI) {
  auto service = ServerStoreService<NoUsersContext>{};
  EXPECT_THROW({auto user = service.CurrentUserInfo();}, Exception);
}

TEST(ServerStoreService, TooManyUsers) {
  auto service = ServerStoreService<TooManyUsersContext>{};
  EXPECT_THROW({auto user = service.CurrentUserInfo();}, Exception);
}

TEST(ServerStoreService, FindOneUser) {
  static constexpr char goodHash[] = "goodHash";
  auto service = ServerStoreService<FindOneUserContext>{};
  FindOneUserContext::goodHash = goodHash;
  auto user = service.CurrentUserInfo();
  EXPECT_EQ(user.id, goodHash);
}

TEST(ServerStoreService, EmptyJwtThrows) {
  auto service = ServerStoreService<EmptyJwtContext>{};
  UserInfo user{.id = "my@name.com"};
  EXPECT_THROW(
      {
        auto jwt = service.GenerateUserJwt("this-is-a-web-token", user);
      },
      Exception);
}

TEST(ServerStoreService, NonEmptyJwtNeverThrows) {
  auto service = ServerStoreService<IdentityJwtContext>{};
  UserInfo user{.id = "my@name.com"};
  std::string token{"this-is-a-web-token"};
  auto jwt = service.GenerateUserJwt(token, user);
  EXPECT_EQ(token, jwt);
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
