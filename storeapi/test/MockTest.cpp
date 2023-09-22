#if defined UP4W_TEST_WITH_MS_STORE_MOCK
/// A mini integration test if testing with mock is enabled.

// clang-format off
/// The test cases below assume the storemockserver was run as:
/// ```sh
/// storemockserver -a $UP4W_MS_STORE_MOCK_ENDPOINT ./storeapi/test/testcase.yaml
/// ```
// clang-format on
#include <gtest/gtest.h>
#include <winrt/base.h>

#include <agent/ServerStoreService.hpp>
#include <base/Exception.hpp>
#include <chrono>
#include <cstdint>
#include <gui/ClientStoreService.hpp>
#include <limits>
#include <string>

#include "base/Purchase.hpp"
#include "base/impl/WinMockContext.hpp"

namespace StoreApi {
TEST(Mock, JwtExpiredToken) {
  ServerStoreService<impl::WinMockContext> agentService{};
  EXPECT_THROW(
      {
        auto jwt = agentService.GenerateUserJwt("expiredtoken",
                                                UserInfo{.id = "hello"});
      },
      winrt::hresult_error);
}

TEST(Mock, JwtServerError) {
  ServerStoreService<impl::WinMockContext> agentService{};
  EXPECT_THROW(
      {
        auto jwt = agentService.GenerateUserJwt("servererror",
                                                UserInfo{.id = "hello"});
      },
      winrt::hresult_error);
}

TEST(Mock, JwtSuccess) {
  ServerStoreService<impl::WinMockContext> agentService{};
  auto jwt = agentService.GenerateUserJwt("token", UserInfo{.id = "hello"});
  EXPECT_EQ(jwt, "CPP_MOCK_JWT_from_user_hello");
}

TEST(Mock, AgentSuccess) {
  ServerStoreService<impl::WinMockContext> agentService{};
  auto user = agentService.CurrentUserInfo();
  EXPECT_EQ(user.id, "user@email.pizza");
  auto jwt = agentService.GenerateUserJwt("token", user);
  EXPECT_EQ(jwt, "CPP_MOCK_JWT_from_user_user@email.pizza");
}

TEST(Mock, PurchaseNonExistent) {
  using namespace std::literals::chrono_literals;

  std::string const productID{"nonexistent"};
  ClientStoreService<impl::WinMockContext> guiService{0};
  EXPECT_THROW({ auto p = guiService.FetchAvailableProduct(productID); },
               winrt::hresult_error);
}

TEST(Mock, PurchaseServerError) {
  using namespace std::literals::chrono_literals;

  std::string const productID{"servererror"};
  ClientStoreService<impl::WinMockContext> guiService{0};
  auto p = guiService.FetchAvailableProduct(productID);
  p.PromptUserForPurchase([&](PurchaseStatus st, int32_t err) {
    EXPECT_EQ(err, 0);
    EXPECT_EQ(st, PurchaseStatus::ServerError);
  });
}

TEST(Mock, PurchaseCannotPurchase) {
  using namespace std::literals::chrono_literals;

  std::string const productID{"cannotpurchase"};
  ClientStoreService<impl::WinMockContext> guiService{0};
  auto p = guiService.FetchAvailableProduct(productID);
  p.PromptUserForPurchase([&](PurchaseStatus st, int32_t err) {
    EXPECT_EQ(err, 0);
    EXPECT_EQ(st, PurchaseStatus::UserGaveUp);
  });
}

TEST(Mock, PurchaseSuccess) {
  using namespace std::literals::chrono_literals;

  std::string const productID{"CPP_MOCK_SUBSCRIPTION"};
  constexpr auto minInt64 = std::numeric_limits<int64_t>::min();
  auto const unsubscribedDate =
      std::chrono::system_clock::from_time_t(minInt64);

  auto const now = std::chrono::system_clock::now();
  auto const later = now + std::chrono::years(1);
  ClientStoreService<impl::WinMockContext> guiService{0};
  ServerStoreService<impl::WinMockContext> agentService{};
  // The GUI's view of the world
  auto p = guiService.FetchAvailableProduct(productID);
  EXPECT_EQ(p.IsInUserCollection(), false);
  EXPECT_EQ(p.CurrentExpirationDate(), unsubscribedDate);
  // The Agent's
  EXPECT_EQ(agentService.CurrentExpirationDate(productID), minInt64);
  p.PromptUserForPurchase([&](PurchaseStatus st, int32_t err) {
    // post effects of a successful purchase
    EXPECT_EQ(err, 0);
    EXPECT_EQ(st, PurchaseStatus::Succeeded);
    EXPECT_GT(agentService.CurrentExpirationDate(productID),
              std::chrono::system_clock::to_time_t(later));
  });
}
}  // namespace StoreApi
#endif  // UP4W_TEST_WITH_MS_STORE_MOCK
