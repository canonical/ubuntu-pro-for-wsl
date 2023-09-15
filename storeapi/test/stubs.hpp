#pragma once
#include <base/Exception.hpp>
#include <base/Purchase.hpp>
// For WinRT basic types and coroutines.
#include <winrt/windows.foundation.h>
// For non-WinRT coroutines
#include <pplawait.h>

// Win32 APIs, such as the Timezone
#include <windows.h>

#include <functional>
#include <span>
#include <vector>

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
  using Window = char;
  struct Product {
    std::string kind;
    std::string id;

    std::chrono::system_clock::time_point CurrentExpirationDate() const {
      throw std::logic_error{"This should not be called"};
    }

    bool IsInUserCollection() const {
      throw std::logic_error{"This should not be called"};
    }

    void PromptUserForPurchase(StoreApi::PurchaseCallback) {
      throw std::logic_error{"This should not be called"};
    }
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {};
  }

  //winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
  //    winrt::hstring hToken, winrt::hstring hUserId) {
  //  co_return hToken;
  //}

  // noop
  void InitDialogs(Window window) {}
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
    using Window = char;
    struct Product {
      std::string kind;
      std::string id;

      std::chrono::system_clock::time_point CurrentExpirationDate() const {
        return std::chrono::system_clock::now() + std::chrono::days{9};
      }

      bool IsInUserCollection() const { return true; }

      void PromptUserForPurchase(StoreApi::PurchaseCallback) {
        throw std::logic_error{"This should not be called"};
      }
    };

    std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                     std::span<const std::string> ids) const {
      return {Product{.kind = kinds[0], .id = ids[0]}};
    }

    winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
        winrt::hstring hToken, winrt::hstring hUserId) {
      co_return hToken;
    }

    // noop
    void InitDialogs(Window window) {}
  };

  // A Store Context that always finds a valid subscription.
  struct PurchaseSuccessContext {
    using Window = char;
    struct Product {
      std::string kind;
      std::string id;

      std::chrono::system_clock::time_point CurrentExpirationDate() {
        throw std::logic_error{"This should not be called"};
      }

      bool IsInUserCollection() const { return false; }

      void PromptUserForPurchase(StoreApi::PurchaseCallback cb) const {
        cb(StoreApi::PurchaseStatus::Succeeded, 0);
      }
    };

    std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                     std::span<const std::string> ids) const {
      return {Product{.kind = kinds[0], .id = ids[0]}};
    }

    winrt::Windows::Foundation::IAsyncOperation<winrt::hstring> GenerateUserJwt(
        winrt::hstring hToken, winrt::hstring hUserId) {
      co_return hToken;
    }

    // noop
    void InitDialogs(Window window) {}
  };
