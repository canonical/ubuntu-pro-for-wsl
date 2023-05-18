#include <gtest/gtest.h>

#include <base/StoreService.hpp>

#include "stubs.hpp"

namespace StoreApi {

// StoreService is meant to be a base class, so we need to inherit from it in
// order to test it.
class DoubledService : public StoreService<DoubledContext> {
 public:
  using StoreService<DoubledContext>::GetSubscriptionProduct;
};

TEST(StoreService, DoubledProductsThrow) {
  DoubledService service{};
  EXPECT_THROW({ service.GetSubscriptionProduct("never-mind").get(); },
               Exception);
}

class EmptyService : public StoreService<EmptyContext> {
 public:
  using StoreService<EmptyContext>::GetSubscriptionProduct;
};
TEST(StoreService, EmptyProductsThrow) {
  DoubledService service{};
  EXPECT_THROW({ service.GetSubscriptionProduct("never-mind").get(); },
               Exception);
}

class IdentityService : public StoreService<FirstContext> {
 public:
  using StoreService<FirstContext>::GetSubscriptionProduct;
};
TEST(IdentityService, OneProductNoThrow) {
  IdentityService service{};
  auto product = service.GetSubscriptionProduct("never-mind").get();
  EXPECT_EQ(product.kind, L"Durable");
  EXPECT_EQ(product.id, L"never-mind");
}

}  // namespace StoreApi
