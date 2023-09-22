#include <winrt/base.h>

#include <algorithm>
#include <span>
#include <string>
#include <vector>

namespace StoreApi::impl {
// Converts a span of strings into a vector of hstrings, needed when passing
// a collection of string as a parameter to an async operation.
inline std::vector<winrt::hstring> to_hstrings(
    std::span<const std::string> input) {
  std::vector<winrt::hstring> hStrs;
  hStrs.reserve(input.size());
  std::ranges::transform(input, std::back_inserter(hStrs),
                         &winrt::to_hstring<std::string>);
  return hStrs;
}
}  // namespace StoreApi::impl
