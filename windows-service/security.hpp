#pragma once

#include "utility.hpp"

#include <cstddef>
#include <expected>
#include <memory>
#include <string>
#include <windows.h>
#include <sddl.h>
#include <wil/resource.h>

namespace efivar::service {

class SecurityDescriptor {
    SECURITY_ATTRIBUTES sa_{sizeof(sa_), nullptr, FALSE};
    wil::unique_hlocal_ptr<void> sd_;

    static std::expected<SecurityDescriptor, std::error_code> FromSddl(const wchar_t* sddl) {
        SecurityDescriptor result;
        PSECURITY_DESCRIPTOR psd = nullptr;
        if (!ConvertStringSecurityDescriptorToSecurityDescriptorW(
                sddl, SDDL_REVISION_1, &psd, nullptr)) {
            return std::unexpected(last_error());
        }
        result.sd_.reset(psd);
        result.sa_.lpSecurityDescriptor = result.sd_.get();
        return result;
    }

public:
    SecurityDescriptor() = default;

    SecurityDescriptor(const SecurityDescriptor&) = delete;
    SecurityDescriptor& operator=(const SecurityDescriptor&) = delete;

    SecurityDescriptor(SecurityDescriptor&&) = default;
    SecurityDescriptor& operator=(SecurityDescriptor&&) = default;

    SECURITY_ATTRIBUTES* get() noexcept { return &sa_; }
    SECURITY_ATTRIBUTES* operator&() noexcept { return &sa_; }

    static std::expected<SecurityDescriptor, std::error_code> Create() {
        return FromSddl(L"D:(A;;GRGW;;;SY)(A;;GRGW;;;AU)S:(ML;;NW;;;ME)");
    }

    static std::expected<SecurityDescriptor, std::error_code> FromCurrentUser() {
        wil::unique_handle token;
        if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, token.put())) {
            return std::unexpected(last_error());
        }

        DWORD length = 0;
        GetTokenInformation(token.get(), TokenUser, nullptr, 0, &length);
        if (GetLastError() != ERROR_INSUFFICIENT_BUFFER) {
            return std::unexpected(last_error());
        }

        auto buffer = std::make_unique<std::byte[]>(length);
        if (!GetTokenInformation(token.get(), TokenUser, static_cast<LPVOID>(buffer.get()), length, &length)) {
            return std::unexpected(last_error());
        }

        auto* tokenUser = reinterpret_cast<TOKEN_USER*>(buffer.get());
        wchar_t* sidString = nullptr;
        if (!ConvertSidToStringSidW(tokenUser->User.Sid, &sidString)) {
            return std::unexpected(last_error());
        }
        wil::unique_hlocal_ptr<wchar_t> sidGuard(sidString);

        std::wstring sddl = L"D:(A;;GRGW;;;SY)(A;;GRGW;;;";
        sddl += sidString;
        sddl += L")S:(ML;;NW;;;ME)";
        return FromSddl(sddl.c_str());
    }
};

} // namespace efivar::service
