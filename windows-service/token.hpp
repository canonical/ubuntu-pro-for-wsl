#pragma once

#include "utility.hpp"

#include <expected>
#include <windows.h>
#include <wil/resource.h>

namespace efivar::service {

class TokenPrivilege {
    wil::unique_handle token_;
    LUID luid_{};
    bool enabled_ = false;

public:
    TokenPrivilege() = default;
    ~TokenPrivilege() {
        if (enabled_ && token_.is_valid()) {
            TOKEN_PRIVILEGES tp{};
            tp.PrivilegeCount = 1;
            tp.Privileges[0].Luid = luid_;
            tp.Privileges[0].Attributes = 0;
            AdjustTokenPrivileges(token_.get(), FALSE, &tp, sizeof(tp), nullptr, nullptr);
        }
    }

    TokenPrivilege(const TokenPrivilege&) = delete;
    TokenPrivilege& operator=(const TokenPrivilege&) = delete;

    TokenPrivilege(TokenPrivilege&& other) noexcept
        : token_(std::move(other.token_)), luid_(other.luid_), enabled_(other.enabled_) {
        other.enabled_ = false;
    }

    TokenPrivilege& operator=(TokenPrivilege&& other) noexcept {
        if (this != &other) {
            token_ = std::move(other.token_);
            luid_ = other.luid_;
            enabled_ = other.enabled_;
            other.enabled_ = false;
        }
        return *this;
    }

    static std::expected<TokenPrivilege, std::error_code> Acquire(const wchar_t* privilegeName) {
        TokenPrivilege result;
        if (!OpenProcessToken(GetCurrentProcess(), TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY, result.token_.put())) {
            return std::unexpected(last_error());
        }

        if (!LookupPrivilegeValueW(nullptr, privilegeName, &result.luid_)) {
            return std::unexpected(last_error());
        }

        TOKEN_PRIVILEGES tp{};
        tp.PrivilegeCount = 1;
        tp.Privileges[0].Luid = result.luid_;
        tp.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED;

        if (!AdjustTokenPrivileges(result.token_.get(), FALSE, &tp, sizeof(tp), nullptr, nullptr)) {
            return std::unexpected(last_error());
        }

        if (GetLastError() == ERROR_NOT_ALL_ASSIGNED) {
            return std::unexpected(last_error(ERROR_NOT_ALL_ASSIGNED));
        }

        result.enabled_ = true;
        return result;
    }

    bool enabled() const noexcept { return enabled_; }
};

} // namespace efivar::service
