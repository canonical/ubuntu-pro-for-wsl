#pragma once

#include <array>
#include <format>
#include <string>

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
  static constexpr char _productKind[] = "Durable";

  // A blocking operation that returns an instance of [ContextType::Product]
  // subscription add-on matching the provided product [id].
  typename ContextType::Product GetSubscriptionProduct(std::string id) {
    std::array<const std::string, 1> ids{std::move(id)};
    std::array<const std::string, 1> kinds{_productKind};
    auto products = context.GetProducts(kinds, ids);
    auto size = products.size();
    switch (size) {
      case 0:
        throw Exception(ErrorCode::NoProductsFound,
                        std::format("id={}", ids[0]));
      case 1:
        return products[0];
      default:
        throw Exception(
            ErrorCode::TooManyProductsFound,
            std::format("Expected one but found {} products for id {}", size,
                        ids[0]));
    }
  }
};

}  // namespace StoreApi
