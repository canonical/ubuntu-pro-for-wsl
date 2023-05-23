#include <gtest/gtest.h>

#include <gui/ClientStoreService.hpp>

#include "stubs.hpp"

namespace StoreApi {

static constexpr char productId[] = "my-awesome-addon";

TEST(ClientStoreService, CannotRePurchase) {
  auto service = ClientStoreService<AlreadyPurchasedContext>{nullptr};
  auto res = service.PromptUserToSubscribe(productId).get();
  EXPECT_EQ(res, PurchaseStatus::AlreadyPurchased);
}

TEST(ClientStoreService, Success) {
  // or whatever else the underlying purchase operation may return.
  auto service = ClientStoreService<PurchaseSuccessContext>{nullptr};
  auto res = service.PromptUserToSubscribe(productId).get();
  EXPECT_EQ(res, PurchaseStatus::Succeeded);
}

}  // namespace StoreApi
