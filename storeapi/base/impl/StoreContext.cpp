#include "StoreContext.hpp"

#include <winrt/Windows.Foundation.Collections.h>

#include <format>

#include "../Exception.hpp"

namespace StoreApi::impl {

using concurrency::task;
using winrt::Windows::Foundation::AsyncStatus;
using winrt::Windows::Foundation::IAsyncOperation;
using winrt::Windows::Services::Store::StoreProduct;
using winrt::Windows::Services::Store::StoreProductQueryResult;
using winrt::Windows::Services::Store::StorePurchaseResult;
using winrt::Windows::Services::Store::StorePurchaseStatus;
using winrt::Windows::Services::Store::StoreSku;

namespace {
// Converts a span of strings into a vector of hstrings, needed when passing
// a collection of string as a parameter to an async operation.
std::vector<winrt::hstring> to_hstrings(std::span<const std::string> input);

// Translates a [StorePurchaseStatus] into the [PurchaseStatus] enum.
PurchaseStatus translate(StorePurchaseStatus purchaseStatus) noexcept;
}  // namespace

std::chrono::system_clock::time_point
StoreContext::Product::CurrentExpirationDate() const {
  // A single product might have more than one SKU.
  for (auto sku : self.Skus()) {
    if (sku.IsInUserCollection() && sku.IsSubscription()) {
      auto collected = sku.CollectionData();
      return winrt::clock::to_sys(collected.EndDate());
    }
  }

  // Should be unreachable if called from a product user is subscribed to.
  throw Exception{
      ErrorCode::Unsubscribed,
      std::format("product ID: {}", winrt::to_string(self.StoreId())),
  };
}

void StoreContext::Product::PromptUserForPurchase(
    PurchaseCallback callback) const {
  debug_assert(callback, "callback must have a target function");
  self.RequestPurchaseAsync().Completed(
      // The lambda will be called once the RequestPurchaseAsync completes.
      [cb = std::move(callback)](
          IAsyncOperation<StorePurchaseResult> const& async,
          AsyncStatus const& status) {
        // We just translate the results (and/or errors)
        auto res = async.GetResults();
        auto error = res.ExtendedError().value;

        // And run the supplied callback.
        cb(translate(res.Status()), error);
      });
}

std::vector<StoreContext::Product> StoreContext::GetProducts(
    std::span<const std::string> kinds, std::span<const std::string> ids) {
  debug_assert(!kinds.empty(), "kinds vector cannot be empty");
  debug_assert(!ids.empty(), "ids vector cannot be empty");
  // Gets Microsoft Store listing info for the specified products that are
  // associated with the current app. Requires "arrays" of product kinds and
  // ids.
  StoreProductQueryResult query =
      self.GetStoreProductsAsync(to_hstrings(kinds), to_hstrings(ids)).get();
  winrt::check_hresult(query.ExtendedError());

  std::vector<Product> products;
  // Could be empty.
  for (auto p : query.Products()) {
    products.emplace_back(p.Value());
  }
  return products;
}

void StoreContext::InitDialogs(Window parentWindow) {
  // Apps that do not feature a [CoreWindow] must inform the runtime the parent
  // window handle in order to render runtime provided UI elements, such as
  // authorization and purchase dialogs.
  auto iiw = self.as<::IInitializeWithWindow>();
  iiw->Initialize(parentWindow);
}

namespace {
std::vector<winrt::hstring> to_hstrings(std::span<const std::string> input) {
  std::vector<winrt::hstring> hStrs;
  hStrs.reserve(input.size());
  std::ranges::transform(input, std::back_inserter(hStrs),
                         &winrt::to_hstring<std::string>);
  return hStrs;
}

PurchaseStatus translate(StorePurchaseStatus purchaseStatus) noexcept {
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
  return StoreApi::PurchaseStatus::Unknown;  // To be future proof.
}
}  // namespace

}  // namespace StoreApi::impl
