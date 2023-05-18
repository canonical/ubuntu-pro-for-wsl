#pragma once
#include <base/Context.hpp>
#include <base/StoreService.hpp>

namespace StoreApi {

// We'll certainly want to show in the UI the result of the purchase operation
// in a localizable way. Thus we must agree on the values returned across the
// different languages involved. Since we don't control the Windows Runtime
// APIs, it wouldn't be future-proof to return the raw value of
// StorePurchaseStatus enum right away.
enum class PurchaseStatus : std::int8_t {
  Succeeded = 0,
  AlreadyPurchased = 1,
  UserGaveUp = 2,
  NetworkError = 3,
  ServerError = 4,
  Unknown = 5,
};

PurchaseStatus translate(
    winrt::Windows::Services::Store::StorePurchaseStatus purchaseStatus) {
  using winrt::Windows::Services::Store::StorePurchaseStatus;
  switch (purchaseStatus) {
    case StorePurchaseStatus::Succeeded:
      return PurchaseStatus::Succeeded;
    case StorePurchaseStatus::AlreadyPurchased:
      return PurchaseStatus::AlreadyPurchased;
    case StorePurchaseStatus::NotPurchased:
      return PurchaseStatus::UserGaveUp;
    case StorePurchaseStatus::NetworkError:
      return PurchaseStatus::NetworkError;
    case StorePurchaseStatus::ServerError:
      return PurchaseStatus::ServerError;
  }
  return PurchaseStatus::Unknown; // To be future proof.
}

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
