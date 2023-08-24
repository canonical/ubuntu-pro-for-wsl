#pragma once
// For the underlying Store API
#include <winrt/windows.services.store.h>

// To provide coroutines capable of returning non-WinRT types.
#include <pplawait.h>

#include <format>

#include "Exception.hpp"

namespace StoreApi {

// A service base class abstracting the MS Store API capable of providing
// product subscription information about the current user. This must be
// extended for more specific usage.
template <typename ContextType>
class StoreService {
 protected:
  // The underlying store context.
  ContextType context{};
  // We only care about subscription add-ons.
  static constexpr wchar_t _productKind[] = L"Durable";

  // An asynchronous operation that eventually returns an instance of
  // [ContextType::Product] subscription add-on matching the provided product
  // [id]. Callers must ensure the underlying string pointed by the [id]
  // remains valid until this function completes.
  concurrency::task<typename ContextType::Product> GetSubscriptionProduct(
      std::string_view id) {
    auto products =
        co_await context.GetProducts({{_productKind}}, {winrt::to_hstring(id)});
    auto size = products.size();
    switch (size) {
      case 0:
        throw Exception(ErrorCode::NoProductsFound, std::format("id={}", id));
      case 1:
        co_return products[0];
      default:
        throw Exception(
            ErrorCode::TooManyProductsFound,
            std::format("Expected one but found {} products for id {}", size,
                        id));
    }
  }
};

}  // namespace StoreApi
