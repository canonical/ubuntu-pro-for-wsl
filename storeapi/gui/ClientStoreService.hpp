#pragma once
#include <base/Context.hpp>
#include <base/StoreService.hpp>

namespace StoreApi {
// Adds functionality on top of the [StoreService] interesting to Client UI
// applications.
template <typename ContextType = Context>
class ClientStoreService : public StoreService<ContextType> {
 public:
  // Initializes a client store service with the top level window handle so the
  // purchase dialog provided by the runtime can be rendered when needed. It's
  // desirable to have the supplied window handle referring to a stable window,
  // so we don't incur in handle reuse problems. The top level window that
  // doesn't change throughout the app lifetime is the best candidate.
  ClientStoreService(HWND topLevelWindow) {
    this->context.InitDialogs(topLevelWindow);
  }

  // Requests the runtime to display the purchase flow dialog for the
  // [productId].
  concurrency::task<PurchaseStatus> PromptUserToSubscribe(
      std::string productId) {
    auto product = co_await this->GetSubscriptionProduct(productId);
    if (product.IsInUserCollection() &&
        product.CurrentExpirationDate() > winrt::clock::now()) {
      // No need to purchase this, right? It would end up with
      // the following status:
      co_return PurchaseStatus::AlreadyPurchased;
    }

    auto res = co_await product.PromptUserForPurchase();
    co_return translate(res);
  }
};

}  // namespace StoreApi
