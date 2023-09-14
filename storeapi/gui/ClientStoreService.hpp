#pragma once
#include <base/DefaultContext.hpp>
#include <base/Exception.hpp>
#include <base/StoreService.hpp>
#include <chrono>
#include <format>
#include <string>

namespace StoreApi {
// Adds functionality on top of the [StoreService] interesting to Client UI
// applications.
template <typename ContextType = DefaultContext>
class ClientStoreService : public StoreService<ContextType> {
 public:
  // Initializes a client store service with the top level window handle so the
  // purchase dialog provided by the runtime can be rendered when needed. It's
  // desirable to have the supplied window handle referring to a stable window,
  // so we don't incur in handle reuse problems. The top level window that
  // doesn't change throughout the app lifetime is the best candidate.
  explicit ClientStoreService(ContextType::Window topLevelWindow) {
    this->context.InitDialogs(topLevelWindow);
  }

  /// Leverages the type system to promote access to the PromptUserForPurchase()
  /// method on Product, which should not be available on non-GUI clients.
  class AvailableProduct : public ContextType::Product {
    using base = ContextType::Product;

   public:
    using base::PromptUserForPurchase;
    AvailableProduct(base B) : base::Product{B} {}
  };

  /// Fetches a subscription product matching the provided product ID available
  /// for purchase. An Exception is thrown if the product is already purchased
  /// or not found.
  AvailableProduct FetchAvailableProduct(std::string productId) {
    auto product = this->GetSubscriptionProduct(productId);
    if (product.IsInUserCollection() &&
        product.CurrentExpirationDate() > std::chrono::system_clock::now()) {
      throw Exception(ErrorCode::InvalidProductId,
                      std::format("product {} already purchased", productId));
    }
    return {product};
  }
};

}  // namespace StoreApi
