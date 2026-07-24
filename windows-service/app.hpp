#pragma once

#include "eventlog.hpp"
#include "firmware.hpp"
#include "pipe.hpp"
#include "protocol.h"
#include "security.hpp"
#include "token.hpp"
#include "utility.hpp"

#include <appmodel.h>
#include <expected>
#include <string>
#include <vector>
#include <windows.h>
#include <wil/resource.h>

namespace efivar::service {

enum class Mode {
    Service,
    Console
};

inline std::expected<bool, std::error_code> IsSamePackage(HANDLE hPipe) {
    ULONG clientPid = 0;
    if (!GetNamedPipeClientProcessId(hPipe, &clientPid)) {
        return std::unexpected(last_error());
    }

    wil::unique_handle client(OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, clientPid));
    if (!client.is_valid()) {
        return std::unexpected(last_error());
    }

    UINT32 clientLen = 0;
    LONG status = GetPackageFamilyName(client.get(), &clientLen, nullptr);
    if (status != ERROR_INSUFFICIENT_BUFFER || clientLen == 0) {
        return false;
    }

    std::vector<wchar_t> clientName(clientLen);
    if (GetPackageFamilyName(client.get(), &clientLen, clientName.data()) != ERROR_SUCCESS) {
        return std::unexpected(last_error());
    }

    UINT32 ownLen = 0;
    if (GetCurrentPackageFullName(&ownLen, nullptr) != ERROR_INSUFFICIENT_BUFFER) {
        return true; // unpackaged: skip check
    }

    std::vector<wchar_t> ownFull(ownLen);
    if (GetCurrentPackageFullName(&ownLen, ownFull.data()) != ERROR_SUCCESS) {
        return std::unexpected(last_error());
    }

    UINT32 familyLen = 0;
    if (PackageFamilyNameFromFullName(ownFull.data(), &familyLen, nullptr) != ERROR_INSUFFICIENT_BUFFER) {
        return std::unexpected(last_error());
    }

    std::vector<wchar_t> ownFamily(familyLen);
    if (PackageFamilyNameFromFullName(ownFull.data(), &familyLen, ownFamily.data()) != ERROR_SUCCESS) {
        return std::unexpected(last_error());
    }

    return std::wcscmp(clientName.data(), ownFamily.data()) == 0;
}

class ServiceApp {
    static constexpr const wchar_t* VariableName = L"UbuntuToken";
    static constexpr const wchar_t* VariableGuid = L"{4f72e91a-a5b3-4c9d-8a6e-23d57bf4e9ac}";

    wil::unique_event ownedStopEvent_;
    HANDLE stopEventHandle_ = nullptr;
    bool ownsStopEvent_ = true;
    EventLog eventLog_;
    SecurityDescriptor securityDescriptor_;
    Pipe pipe_;
    Mode mode_ = Mode::Console;

    ServiceApp(
        wil::unique_event ownedStopEvent,
        HANDLE stopEventHandle,
        bool ownsStopEvent,
        EventLog eventLog,
        SecurityDescriptor securityDescriptor,
        Pipe pipe,
        Mode mode)
        : ownedStopEvent_(std::move(ownedStopEvent)),
          stopEventHandle_(stopEventHandle),
          ownsStopEvent_(ownsStopEvent),
          eventLog_(std::move(eventLog)),
          securityDescriptor_(std::move(securityDescriptor)),
          pipe_(std::move(pipe)),
          mode_(mode) {}

    void SendError(
        Pipe::Connection& conn,
        efivar::ServiceErrc errc,
        DWORD win32Error = 0) const {
        efivar::Response resp{};
        resp.magic = efivar::MagicValue;
        resp.version = efivar::VersionV1;
        resp.serviceError = static_cast<std::uint32_t>(errc);
        resp.win32Error = win32Error;
        resp.valueLength = 0;
        (void)conn.Write(&resp, sizeof(resp));
    }

public:
    ServiceApp() = default;

    ServiceApp(const ServiceApp&) = delete;
    ServiceApp& operator=(const ServiceApp&) = delete;

    ServiceApp(ServiceApp&&) = default;
    ServiceApp& operator=(ServiceApp&&) = default;

    static std::expected<ServiceApp, std::error_code> Initialize(Mode mode, HANDLE existingStopEvent = nullptr) {
        wil::unique_event ownedStopEvent;
        HANDLE stopEventHandle = existingStopEvent;
        bool ownsStopEvent = false;

        if (!existingStopEvent) {
            ownedStopEvent.reset(CreateEventW(nullptr, TRUE, FALSE, nullptr));
            if (!ownedStopEvent.is_valid()) {
                return std::unexpected(last_error());
            }
            stopEventHandle = ownedStopEvent.get();
            ownsStopEvent = true;
        }

        auto eventLog = EventLog::Open(L"EfivarService");
        if (!eventLog) {
            return std::unexpected(eventLog.error());
        }

        auto securityDescriptor = SecurityDescriptor::Create();
        if (!securityDescriptor) {
            return std::unexpected(securityDescriptor.error());
        }

        auto pipe = Pipe::Create(efivar::PipeName, securityDescriptor->get());
        if (!pipe) {
            eventLog->Error(2, L"CreateNamedPipe failed; error " + std::to_wstring(pipe.error().value()));
            return std::unexpected(pipe.error());
        }

        return ServiceApp(
            std::move(ownedStopEvent),
            stopEventHandle,
            ownsStopEvent,
            std::move(*eventLog),
            std::move(*securityDescriptor),
            std::move(*pipe),
            mode);
    }

    HANDLE stop_event() const noexcept { return stopEventHandle_; }

    void ProcessRequest(Pipe::Connection& conn) const {
        auto requestResult = conn.Read<efivar::Request>();
        if (!requestResult) {
            return;
        }
        const auto& request = *requestResult;

        eventLog_.Info(3, L"Request received");

        if (request.magic != efivar::MagicValue || request.version != efivar::VersionV1) {
            SendError(conn, efivar::ServiceErrc::BadRequest);
            return;
        }

        if (request.command == efivar::CMD_READ) {
            auto privilege = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
            if (!privilege) {
                SendError(conn, efivar::ServiceErrc::PrivilegeFailed, privilege.error().value());
                eventLog_.Warning(5, L"Privilege enable failed; error " + std::to_wstring(privilege.error().value()));
                return;
            }

            auto firmware = Firmware::Read(VariableName, VariableGuid);
            if (!firmware) {
                SendError(conn, efivar::ServiceErrc::FirmwareReadFailed, firmware.error().value());
                eventLog_.Warning(6, L"Firmware read failed; error " + std::to_wstring(firmware.error().value()));
                return;
            }

            const auto& buffer = firmware->buffer;
            DWORD bytesRead = firmware->bytesRead;

            efivar::Response resp{};
            resp.magic = efivar::MagicValue;
            resp.version = efivar::VersionV1;
            resp.serviceError = static_cast<std::uint32_t>(efivar::ServiceErrc::Success);
            resp.win32Error = 0;
            resp.valueLength = bytesRead;

            if (!conn.Write(&resp, sizeof(resp))) {
                return;
            }
            if (resp.valueLength > 0) {
                if (!conn.Write(buffer.data(), resp.valueLength)) {
                    return;
                }
            }

            std::wstring msg;
            int needed = MultiByteToWideChar(
                CP_UTF8, 0,
                buffer.data(),
                static_cast<int>(bytesRead),
                nullptr, 0);
            if (needed > 0) {
                msg.resize(static_cast<size_t>(needed));
                MultiByteToWideChar(
                    CP_UTF8, 0,
                    buffer.data(),
                    static_cast<int>(bytesRead),
                    msg.data(), needed);
            }
            eventLog_.Info(4, L"Request completed with contents \"" + msg + L"\"");
        } else if (request.command == efivar::CMD_LIST) {
            auto privilege = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
            if (!privilege) {
                SendError(conn, efivar::ServiceErrc::PrivilegeFailed, privilege.error().value());
                eventLog_.Warning(5, L"Privilege enable failed; error " + std::to_wstring(privilege.error().value()));
                return;
            }
            auto firmware = Firmware::Enumerate();
            if (!firmware) {
                SendError(conn, efivar::ServiceErrc::FirmwareReadFailed, static_cast<DWORD>(firmware.error().value()));
                eventLog_.Warning(6, L"Firmware enumeration failed; error " + std::to_wstring(firmware.error().value()));
                return;
            }

            const auto& buffer = firmware->buffer;
            ULONG bytesRead = firmware->bytesRead;

            efivar::Response resp{};
            resp.magic = efivar::MagicValue;
            resp.version = efivar::VersionV1;
            resp.serviceError = static_cast<std::uint32_t>(efivar::ServiceErrc::Success);
            resp.win32Error = 0;
            resp.valueLength = bytesRead;

            if (!conn.Write(&resp, sizeof(resp))) {
                return;
            }
            if (resp.valueLength > 0) {
                if (!conn.Write(buffer.data(), resp.valueLength)) {
                    return;
                }
            }

            eventLog_.Info(4, L"Request completed (list)");
        } else {
            SendError(conn, efivar::ServiceErrc::BadRequest);
        }
    }

    void Run() {
        eventLog_.Info(1, mode_ == Mode::Service ? L"Service started" : L"Service started (console mode)");

        while (true) {
            auto connResult = pipe_.Accept(stopEventHandle_);
            if (!connResult) {
                break;
            }
            auto& conn = *connResult;

            auto samePackage = IsSamePackage(pipe_.get());
            if (!samePackage) {
                SendError(conn, efivar::ServiceErrc::ConnectionRefused);
                continue;
            }
            if (!*samePackage) {
                SendError(conn, efivar::ServiceErrc::ConnectionRefused);
                continue;
            }

            ProcessRequest(conn);
        }

        eventLog_.Info(7, mode_ == Mode::Service ? L"Service stopping" : L"Service stopping (console mode)");
    }
};

} // namespace efivar::service
