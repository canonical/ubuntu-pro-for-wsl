#pragma once

/// Test stubs and doubles.

#include <base/Exception.hpp>
#include <base/Purchase.hpp>
#include <chrono>
#include <functional>
#include <span>
#include <stdexcept>
#include <string>
#include <vector>
// For timegm
#include <time.h>

#if defined _MSC_VER
#include <windows.h>
#define timegm _mkgmtime
#endif

// A Store Context that always finds more than  one product
struct DoubledContext {
  struct Product {};

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{}, Product{}, Product{}};
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

  // noop
  void InitDialogs(Window window) {}
};

// A Store Context that always finds exactly one product.
struct FirstContext {
  struct Product {
    std::string kind;
    std::string id;
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{.kind = kinds[0], .id = ids[0]}};
  }
};

// A Store Context that always finds exactly one product.
struct EmptyJwtContext {
  struct Product {
    std::string kind;
    std::string id;
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return {};
  }
};

// A Store Context that always finds exactly one product.
struct IdentityJwtContext {
  struct Product {
    std::string kind;
    std::string id;
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return hToken;
  }
};

// A Store Context that only finds exactly a product user doesn't own.
struct NeverSubscribedContext {
  struct Product {
    std::string kind;
    std::string id;

    bool IsInUserCollection() { return false; }

    std::chrono::system_clock::time_point CurrentExpirationDate() {
      throw StoreApi::Exception{StoreApi::ErrorCode::Unsubscribed,
                                std::format("id: {}", id)};
    }
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return hToken;
  }
};

// A Store Context that always finds a subscription that expired in the Unix
// epoch (in the local time zone).
struct UnixEpochContext {
  struct Product {
    std::string kind;
    std::string id;

    std::chrono::system_clock::time_point CurrentExpirationDate() {
      using namespace std::chrono;
      return sys_days{January / 1 / 1970};
    }

    bool IsInUserCollection() { return true; }
  };

  std::vector<Product> GetProducts(std::span<const std::string> kinds,
                                   std::span<const std::string> ids) const {
    return {Product{.kind = kinds[0], .id = ids[0]}};
  }

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return hToken;
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

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return hToken;
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

  std::string GenerateUserJwt(std::string hToken, std::string hUserId) const {
    return hToken;
  }

  // noop
  void InitDialogs(Window window) {}
};

struct TooManyUsersContext {
  struct Product {};
  std::vector<std::string> AllLocallyAuthenticatedUserHashes() const {
    return {"first-user", "second-user"};
  }
};

struct NoUsers {
  struct Product {};
  std::vector<std::string> AllLocallyAuthenticatedUserHashes() const {
    return {};
  }
};

struct FindOneUser {
  struct Product {};
  static inline std::string goodHash{};
  std::vector<std::string> AllLocallyAuthenticatedUserHashes() const {
    return {goodHash};
  }
};
