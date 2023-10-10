#ifndef UP4W_TEST_WITH_MS_STORE_MOCK

#include "StoreContext.hpp"

#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.Security.Cryptography.core.h>
#include <winrt/Windows.Security.Cryptography.h>
#include <winrt/Windows.Services.Store.h>
#include <winrt/Windows.System.h>
#include <winrt/base.h>

#include <format>
#include <functional>
#include <iterator>
#include <type_traits>

#include "../Exception.hpp"
#include "WinRTHelpers.hpp"

namespace StoreApi::impl {

using winrt::Windows::Foundation::AsyncStatus;
using winrt::Windows::Foundation::IAsyncOperation;
using winrt::Windows::Services::Store::StoreProduct;
using winrt::Windows::Services::Store::StoreProductQueryResult;
using winrt::Windows::Services::Store::StorePurchaseResult;
using winrt::Windows::Services::Store::StorePurchaseStatus;
using winrt::Windows::Services::Store::StoreSku;

namespace {
// Translates a [StorePurchaseStatus] into the [PurchaseStatus] enum.
PurchaseStatus translate(StorePurchaseStatus purchaseStatus) noexcept;

// Returns a hstring representation of a SHA256 sum of the input hstring.
winrt::hstring sha256(winrt::hstring input);
}  // namespace

std::chrono::system_clock::time_point
StoreContext::Product::CurrentExpirationDate() const {
  // A single product might have more than one SKU and not all of them
  // (maybe none) show both `IsSubscription` and `IsInUserCollection` properties
  // simultaneously true.
  for (auto sku : self.Skus()) {
    if (sku.IsInUserCollection()) {
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
  assert(callback && "callback must have a target function");
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
    std::span<const std::string> kinds,
    std::span<const std::string> ids) const {
  assert(!kinds.empty() && "kinds vector cannot be empty");
  assert(!ids.empty() && "ids vector cannot be empty");
  // Gets Microsoft Store listing info for the specified products that are
  // associated with the current app. Requires "arrays" of product kinds and
  // ids.
  StoreProductQueryResult query =
      self.GetStoreProductsAsync(to_hstrings(kinds), to_hstrings(ids)).get();
  winrt::check_hresult(query.ExtendedError());

  std::vector<StoreContext::Product> products;
  // Could be empty.
  for (auto p : query.Products()) {
    products.emplace_back(p.Value());
  }
  return products;
}

std::string StoreContext::GenerateUserJwt(std::string token,
                                          std::string userId) const {
  assert(!token.empty() && "Azure AD token is required");
  auto hJwt = self.GetCustomerPurchaseIdAsync(winrt::to_hstring(token),
                                              winrt::to_hstring(userId))
                  .get();
  return winrt::to_string(hJwt);
}

void StoreContext::InitDialogs(Window parentWindow) {
  // Apps that do not feature a [CoreWindow] must inform the runtime the parent
  // window handle in order to render runtime provided UI elements, such as
  // authorization and purchase dialogs.
  auto iiw = self.as<::IInitializeWithWindow>();
  iiw->Initialize(parentWindow);
}

std::vector<std::string> StoreContext::AllLocallyAuthenticatedUserHashes() {
  using winrt::Windows::Foundation::IInspectable;
  using winrt::Windows::System::KnownUserProperties;
  using winrt::Windows::System::User;
  using winrt::Windows::System::UserAuthenticationStatus;
  using winrt::Windows::System::UserType;

  // This should really return a single user, but the API is specified in terms
  // of a collection, so let's not assume too much.
  auto users =
      User::FindAllAsync(UserType::LocalUser,
                         UserAuthenticationStatus::LocallyAuthenticated)
          .get();

  std::vector<std::string> allHashes;
  allHashes.reserve(users.Size());
  for (auto user : users) {
    IInspectable accountName =
        user.GetPropertyAsync(KnownUserProperties::AccountName()).get();
    auto name = winrt::unbox_value<winrt::hstring>(accountName);
    if (!name.empty()) {
      allHashes.push_back(winrt::to_string(sha256(name)));
    }
  }

  return allHashes;
}

namespace {
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
  assert(false && "Missing enum elements to translate StorePurchaseStatus.");
  return StoreApi::PurchaseStatus::Unknown;  // To be future proof.
}

winrt::hstring sha256(winrt::hstring input) {
  using winrt::Windows::Security::Cryptography::BinaryStringEncoding;
  using winrt::Windows::Security::Cryptography::CryptographicBuffer;
  using winrt::Windows::Security::Cryptography::Core::HashAlgorithmNames;
  using winrt::Windows::Security::Cryptography::Core::HashAlgorithmProvider;

  auto inputUtf8 = CryptographicBuffer::ConvertStringToBinary(
      winrt::to_hstring(input), BinaryStringEncoding::Utf8);
  auto hasher =
      HashAlgorithmProvider::OpenAlgorithm(HashAlgorithmNames::Sha256());
  return CryptographicBuffer::EncodeToHexString(hasher.HashData(inputUtf8));
}

}  // namespace

}  // namespace StoreApi::impl

#endif  // UP4W_TEST_WITH_MS_STORE_MOCK
