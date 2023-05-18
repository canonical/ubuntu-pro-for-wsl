#pragma once
#include <stdexcept>

namespace StoreApi {
// Custom Exception type reporting our business logic errors (not translating
// winrt::hresult_error's).
struct Exception : public std::runtime_error {
  using std::runtime_error::runtime_error;
};
}  // namespace StoreApi
