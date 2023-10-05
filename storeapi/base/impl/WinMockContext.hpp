#pragma once

/// Implements a replacement for [StoreContext] which talks to the MS Store Mock
/// Server (still on Windows) instead of the real MS APIs.
/// DO NOT USE IN PRODUCTION.
#if defined UP4W_TEST_WITH_MS_STORE_MOCK && defined _MSC_VER

#include <chrono>
#include <span>
#include <string>
#include <type_traits>
#include <vector>

#include "../Purchase.hpp"

namespace winrt::Windows::Data::Json {
struct JsonObject;
}

namespace StoreApi::impl {

class WinMockContext {
 public:
  using Window = std::int32_t;
  class Product {
    std::string storeID;
    std::string title;
    std::string description;
    std::string productKind;
    std::chrono::system_clock::time_point expirationDate;
    bool isInUserCollection;

   public:
    // Whether the current user owns this product.
    bool IsInUserCollection() const { return isInUserCollection; }

    // Assuming this is a Subscription add-on product the current user __owns__,
    // returns the expiration date of the current billing period.
    std::chrono::system_clock::time_point CurrentExpirationDate() const {
      return expirationDate;
    }

   protected:
    // Assuming this is a Subscription add-on product the current user __does
    // not own__, requests the runtime to display a purchase flow so users can
    // subscribe to this product. THis function returns early, the result will
    // eventually arrive through the supplied callback. This must be called from
    // a UI thread with the underlying store context initialized with the parent
    // GUI window because we need to render native dialogs. Thus, access to this
    // method must be protected so we can ensure it can only happen with GUI
    // clients, making API misuse harder to happen.
    // See
    // https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storeproduct.requestpurchaseasync
    void PromptUserForPurchase(PurchaseCallback callback) const;

   public:
    /// Creates a product from a JsonObject obtained from a call to the mock
    /// server containing the relevant information.
    explicit Product(winrt::Windows::Data::Json::JsonObject const& json);
    Product() = default;
  };

  // Returns a collection of products matching the supplied [kinds] and [ids].
  // Ids must match the Product IDs in Partner Center. Kinds can be:
  // Application; Game; Consumable; UnmanagedConsumable; Durable. See
  // https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storeproduct.productkind#remarks
  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const;

  // Generates the user ID key (a.k.a the JWT) provided the server AAD [token]
  // and the [userId] the caller wants to have encoded in the JWT.
  std::string GenerateUserJwt(std::string token, std::string userId) const;

  // Initializes the GUI "subsystem" with the [parentWindow] handle so we can
  // render native dialogs, such as when purchase or other kinds of
  // authorization are required.
  void InitDialogs(Window parentWindow);

  // Returns a collection of hashes of all locally authenticated users running
  // in this session. Most likely the collection will contain a single element.
  static std::vector<std::string> AllLocallyAuthenticatedUserHashes();
};

}  // namespace StoreApi::impl

namespace StoreApi {
using DefaultContext = impl::WinMockContext;
}

#endif  // UP4W_TEST_WITH_MS_STORE_MOCK && _MSC_VER
