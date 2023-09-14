#pragma once

/// Here lies the classes wrapping the MS API for testability.
/// Thus this code is inherently non-testable.

// For the underlying Store API
#include <winrt/windows.services.store.h>

// To provide the WinRT coroutine types.
#include <winrt/windows.foundation.h>

// To provide coroutines capable of returning more complex non-WinRT types.
#include <pplawait.h>

// For HWND and GUI-related Windows types.
#include <ShObjIdl.h>

namespace StoreApi::impl {

// Wraps MS StoreContext type for testability purposes.
class StoreContext {
  winrt::Windows::Services::Store::StoreContext self =
      winrt::Windows::Services::Store::StoreContext::GetDefault();

 public:
  // Wraps MS StoreProduct type for testability purposes. This is not meant for
  // direct usage in high level code. The API is loose, the caller services must
  // tighten it up.
  struct Product {
   public:
    Product(winrt::Windows::Services::Store::StoreProduct self) : self{self} {}
    // Whether the current user owns this product.
    bool IsInUserCollection() { return self.IsInUserCollection(); }

    // Assuming this is a Subcription add-on product the current user __owns__,
    // returns the expiration date of the current billing period.
    winrt::Windows::Foundation::DateTime CurrentExpirationDate();

   protected:
    // Assuming this is a Subcription add-on product the current user __does not
    // own__, requests the runtime to display a purchase flow so users can
    // subscribe to this product. This must be called from a UI thread with the
    // underlying store context initialized with the parent GUI window because
    // we need to render native dialogs. See
    // https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storeproduct.requestpurchaseasync
    winrt::Windows::Foundation::IAsyncOperation<
        winrt::Windows::Services::Store::StorePurchaseStatus>
    PromptUserForPurchase();

   private:
    winrt::Windows::Services::Store::StoreProduct self{nullptr};
  };

  // Returns a collection of products matching the supplied [kinds] and [ids].
  // Ids must match the Product IDs in Partner Center. Kinds can be:
  // Application; Game; Consumable; UnmanagedConsumable; Durable. See
  // https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storeproduct.productkind#remarks
  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids);

  // Generates the user ID key (a.k.a the JWT) provided the server AAD [hToken]
  // and the [hUserId] the caller wants to have encoded in the JWT.
  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    return self.GetCustomerPurchaseIdAsync(hToken, hUserId);
  }

  // Initializes the GUI "subsystem" with the [parentWindow] handle so we can
  // render native dialogs, such as when purchase or other kinds of
  // authorization are required.
  void InitDialogs(HWND parentWindow);
};

}  // namespace StoreApi::impl
