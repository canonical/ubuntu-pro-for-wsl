#pragma once

#include <cstdint>
#include <system_error>
#include <string>

namespace efivar {

enum class ServiceErrc : std::uint32_t {
    Success = 0,
    PrivilegeFailed = 1,
    FirmwareReadFailed = 2,
    BadRequest = 3,
    ConnectionRefused = 4
};

class ServiceErrorCategory : public std::error_category {
public:
    const char* name() const noexcept override {
        return "efivar-service";
    }

    std::string message(int ev) const override {
        switch (static_cast<ServiceErrc>(ev)) {
            case ServiceErrc::Success:
                return "success";
            case ServiceErrc::PrivilegeFailed:
                return "failed to enable SeSystemEnvironmentPrivilege";
            case ServiceErrc::FirmwareReadFailed:
                return "failed to read UEFI firmware variable";
            case ServiceErrc::BadRequest:
                return "bad request";
            case ServiceErrc::ConnectionRefused:
                return "connection refused";
            default:
                return "unknown efivar-service error";
        }
    }
};

inline const std::error_category& service_category() noexcept {
    static const ServiceErrorCategory category;
    return category;
}

inline std::error_code make_error_code(ServiceErrc errc) noexcept {
    return std::error_code(static_cast<int>(errc), service_category());
}

#pragma pack(push, 1)
struct Request {
    std::uint16_t magic;
    std::uint16_t version;
    std::uint32_t command;
};

struct Response {
    std::uint16_t magic;
    std::uint16_t version;
    std::uint32_t serviceError;
    std::uint32_t win32Error;
    std::uint32_t valueLength;
};
#pragma pack(pop)

constexpr std::uint16_t MagicValue = 0x5645;
constexpr std::uint16_t VersionV1 = 1;
constexpr std::uint32_t CMD_READ = 0;
constexpr std::uint32_t CMD_LIST = 1;
constexpr wchar_t PipeName[] = L"\\\\.\\pipe\\LOCAL\\efivar-service";

} // namespace efivar

namespace std {

template<>
struct is_error_code_enum<efivar::ServiceErrc> : true_type {};

} // namespace std
