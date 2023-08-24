#pragma once
#include <base/Exception.hpp>
// For WinRT basic types and coroutines.
#include <winrt/windows.foundation.h>
// For non-WinRT coroutines
#include <pplawait.h>

// Win32 APIs, such as the Timezone
#include <windows.h>
// Test stubs and doubles.

// A Store Context that always finds more than  one product
struct DoubledContext {
  struct Product {};

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{}, Product{}, Product{}};
  }
};

// A Store Context that never finds a product.
struct EmptyContext {
  struct Product {};

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {};
  }
};

// A Store Context that always finds exactly one product.
struct FirstContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }
};

// A Store Context that always finds exactly one product.
struct EmptyJwtContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return {};
  }
};

// A Store Context that always finds exactly one product.
struct IdentityJwtContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return hToken;
  }
};

// A Store Context that only finds exactly a product user doesn't own.
struct NeverSubscribedContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;

    bool IsInUserCollection() { return false; }

    winrt::Windows::Foundation::DateTime CurrentExpirationDate() {
      throw StoreApi::Exception{StoreApi::ErrorCode::Unsubscribed,
                                std::format("id: {}", winrt::to_string(id))};
    }
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return hToken;
  }
};

// A Store Context that always finds a subscription that expired in the Unix
// epoch (in the local time zone).
struct UnixEpochContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;

    winrt::Windows::Foundation::DateTime CurrentExpirationDate() {
      TIME_ZONE_INFORMATION tz{};
      std::int64_t seconds = 0;
      if (GetTimeZoneInformation(&tz) != TIME_ZONE_ID_INVALID) {
        // UTC = local time + Bias (in minutes)
        seconds = static_cast<std::int64_t>(tz.Bias) * 60LL;
      }

      return winrt::clock::from_time_t(seconds);  // should be the UNIX epoch.
    }

    bool IsInUserCollection() { return true; }
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return hToken;
  }
};

// A Store Context that always finds a valid subscription.
struct AlreadyPurchasedContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;

    winrt::Windows::Foundation::DateTime CurrentExpirationDate() {
      return winrt::clock::now() + std::chrono::days{9};
    }

    bool IsInUserCollection() { return true; }

    winrt::Windows::Foundation::IAsyncOperation<
        winrt::Windows::Services::Store::StorePurchaseStatus>
    PromptUserForPurchase() {
      throw std::logic_error{"This should not be called"};
    }
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return hToken;
  }

  void InitDialogs(HWND window) { /*nopp*/
  }
};

// A Store Context that always finds a valid subscription.
struct PurchaseSuccessContext {
  struct Product {
    winrt::hstring kind;
    winrt::hstring id;

    winrt::Windows::Foundation::DateTime CurrentExpirationDate() {
      throw std::logic_error{"This should not be called"};
    }

    bool IsInUserCollection() { return false; }

    winrt::Windows::Foundation::IAsyncOperation<
        winrt::Windows::Services::Store::StorePurchaseStatus>
    PromptUserForPurchase() {
      co_return winrt::Windows::Services::Store::StorePurchaseStatus::Succeeded;
    }
  };

  concurrency::task<std::vector<Product>> GetProducts(
      std::vector<winrt::hstring> kinds, std::vector<winrt::hstring> ids) {
    co_return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
      winrt::hstring hToken, winrt::hstring hUserId) {
    co_return hToken;
  }

  void InitDialogs(HWND window) { /*nopp*/
  }
};
