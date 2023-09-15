#include <gtest/gtest.h>

#include <base/Exception.hpp>
#include <gui/ClientStoreService.hpp>

#include "stubs.hpp"

namespace StoreApi {

static constexpr char productId[] = "my-awesome-addon";

TEST(ClientStoreService, ProductNotFound) {
  auto service = ClientStoreService<EmptyContext>{0};
  EXPECT_THROW({ auto prod = service.FetchAvailableProduct(productId); },
               Exception);
}

TEST(ClientStoreService, CannotRePurchase) {
  auto service = ClientStoreService<AlreadyPurchasedContext>{0};
  EXPECT_THROW({ auto p = service.FetchAvailableProduct(productId); },
               Exception);
}

TEST(ClientStoreService, Success) {
  // or whatever else the underlying purchase operation may return.
  auto service = ClientStoreService<PurchaseSuccessContext>{0};
  auto p = service.FetchAvailableProduct(productId);
  p.PromptUserForPurchase([](PurchaseStatus res, std::int32_t hr) {
    EXPECT_EQ(res, PurchaseStatus::Succeeded);
    EXPECT_EQ(hr, 0);
  });
}

}  // namespace StoreApi
