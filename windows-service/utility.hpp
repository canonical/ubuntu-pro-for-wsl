#pragma once

#include <system_error>
#include <windows.h>

namespace efivar::service {

inline std::error_code last_error() noexcept {
    return std::error_code(static_cast<int>(GetLastError()), std::system_category());
}

inline std::error_code last_error(DWORD code) noexcept {
    return std::error_code(static_cast<int>(code), std::system_category());
}

} // namespace efivar::service
