#include "StoreContext.hpp"

#include <winrt/Windows.Foundation.Collections.h>

#include <format>

#include "../Exception.hpp"

namespace StoreApi::impl {

using concurrency::task;
using winrt::Windows::Foundation::DateTime;
using winrt::Windows::Foundation::IAsyncOperation;
using winrt::Windows::Services::Store::StoreProduct;
using winrt::Windows::Services::Store::StoreProductQueryResult;
using winrt::Windows::Services::Store::StorePurchaseStatus;
using winrt::Windows::Services::Store::StoreSku;

DateTime StoreContext::Product::CurrentExpirationDate() {
  // A single product might have more than one SKU.
  for (auto sku : self.Skus()) {
    if (sku.IsInUserCollection() && sku.IsSubscription()) {
      auto collected = sku.CollectionData();
      return collected.EndDate();
    }
  }

  // Should be unreachable if called from a product user is subscribed to.
  throw Exception{
      ErrorCode::Unsubscribed,
      std::format("product ID: {}", winrt::to_string(self.StoreId())),
  };
}

IAsyncOperation<StorePurchaseStatus> StoreContext::Product::PromptUserForPurchase() {
  const auto& res = co_await self.RequestPurchaseAsync();
  // throws winrt::hresult_error if query contains an error HRESULT.
  winrt::check_hresult(res.ExtendedError());
  co_return res.Status();
}

task<std::vector<StoreContext::Product>> StoreContext::GetProducts(
    std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
  // Gets Microsoft Store listing info for the specified products that are
  // associated with the current app. Requires "arrays" of product kinds and
  // ids.
  StoreProductQueryResult query =
      co_await self.GetStoreProductsAsync(std::move(kinds), std::move(ids));
  winrt::check_hresult(query.ExtendedError());

  std::vector<Product> products;
  // Could be empty.
  for (auto p : query.Products()) {
    products.emplace_back(p.Value());
  }
  co_return products;
}

void StoreContext::InitDialogs(HWND parentWindow) {
  // Apps that do not feature a [CoreWindow] must inform the runtime the parent
  // window handle in order to render runtime provided UI elements, such as
  // authorization and purchase dialogs.
  auto iiw = self.as<::IInitializeWithWindow>();
  iiw->Initialize(parentWindow);
}

}  // namespace StoreApi::impl
