#pragma once
#include <exception>
#include <format>
#include <source_location>
#include <string>
#include <type_traits>

namespace StoreApi {

enum class ErrorCode {
  // Domain errors:
  Unsubscribed = -128,
  NoProductsFound,
  TooManyProductsFound,
  InvalidUserInfo,
  NoLocalUser,
  TooManyLocalUsers,
  EmptyJwt,
  InvalidProductId,
  // ABI Boundary errors:
  AllocationFailure = -10,
  //   - input string argument errors
  NullInputPtr = -9,
  TooBigLength = -8,
  ZeroLength = -7,
  //   - output parameter errors
  NullOutputPtr = -6,
  //   - other runtime errors (aka exceptions)
  WinRT = -2,
  Unknown = -1,
  // Not an error.
  None = 0,
};

inline std::string to_string(ErrorCode err) {
  switch (err) {
    case ErrorCode::Unsubscribed:
      return "Current user not subscribed to this product.";
    case ErrorCode::NoProductsFound:
      return "Query found no products.";
    case ErrorCode::TooManyProductsFound:
      return "Query found too many products.";
    case ErrorCode::NoLocalUser:
      return "No locally authenticated user could be found.";
    case ErrorCode::InvalidUserInfo:
      return "Invalid user info. Maybe not a real user session.";
    case ErrorCode::TooManyLocalUsers:
      return "Too many locally authenticated users.";
    case ErrorCode::EmptyJwt:
      return "Empty user JWT was generated.";
    case ErrorCode::Unknown:
      return "Unknown.";
    case ErrorCode::None:
      return "";
  }

  return {};
}

// Custom Exception type reporting our business logic errors (not translating
// winrt::hresult_error's).
class Exception {
  ErrorCode m_code;
  std::string m_detail;
  std::source_location m_loc;

 public:
  explicit Exception(ErrorCode code, std::string&& detail = {},
                     std::source_location loc = std::source_location::current())
      : m_code{code}, m_detail{std::move(detail)}, m_loc{loc} {}

  ErrorCode code() const noexcept { return m_code; }

  std::string what() const {
    return std::format("[ERROR]: {} {}\n{}:{} {}", to_string(m_code), m_detail,
                       m_loc.file_name(), m_loc.line(), m_loc.function_name());
  }
};

}  // namespace StoreApi
