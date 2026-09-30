#pragma once

#include "utility.hpp"

#include <expected>
#include <functional>
#include <windows.h>
#include <wil/resource.h>

namespace efivar::service {

class ServiceControl {
    struct Context {
        wil::unique_event stopEvent;
        SERVICE_STATUS_HANDLE statusHandle = nullptr;
    };

    inline static std::function<void(HANDLE)> s_callback;

    static void ReportStatus(Context& ctx, DWORD currentState, DWORD win32ExitCode, DWORD waitHint) {
        SERVICE_STATUS status{};
        status.dwServiceType = SERVICE_WIN32_OWN_PROCESS;
        status.dwCurrentState = currentState;
        status.dwWin32ExitCode = win32ExitCode;
        status.dwWaitHint = waitHint;
        status.dwControlsAccepted =
            (currentState == SERVICE_START_PENDING) ? 0 : SERVICE_ACCEPT_STOP;

        if (currentState == SERVICE_RUNNING || currentState == SERVICE_STOPPED) {
            status.dwCheckPoint = 0;
        } else {
            static DWORD checkPoint = 0;
            status.dwCheckPoint = ++checkPoint;
        }

        SetServiceStatus(ctx.statusHandle, &status);
    }

    static DWORD WINAPI Handler(
        DWORD control,
        DWORD /*eventType*/,
        LPVOID /*eventData*/,
        LPVOID context) noexcept {
        auto* ctx = static_cast<Context*>(context);
        switch (control) {
            case SERVICE_CONTROL_STOP:
                SetEvent(ctx->stopEvent.get());
                ReportStatus(*ctx, SERVICE_RUNNING, NO_ERROR, 0);
                return NO_ERROR;
            case SERVICE_CONTROL_INTERROGATE:
                return NO_ERROR;
            default:
                return ERROR_CALL_NOT_IMPLEMENTED;
        }
    }

    static void WINAPI ServiceMain(DWORD /*argc*/, LPWSTR* /*argv*/) noexcept {
        Context ctx;
        ctx.stopEvent.reset(CreateEventW(nullptr, TRUE, FALSE, nullptr));
        if (!ctx.stopEvent.is_valid()) {
            return;
        }

        ctx.statusHandle = RegisterServiceCtrlHandlerExW(L"EfivarService", Handler, &ctx);
        if (!ctx.statusHandle) {
            return;
        }

        ReportStatus(ctx, SERVICE_START_PENDING, NO_ERROR, 0);
        ConfigureRestartActions(L"EfivarService");
        ReportStatus(ctx, SERVICE_RUNNING, NO_ERROR, 0);

        if (s_callback) {
            s_callback(ctx.stopEvent.get());
        }

        ReportStatus(ctx, SERVICE_STOPPED, NO_ERROR, 0);
    }

public:
    static int Run(const wchar_t* name, std::function<void(HANDLE)> callback) {
        s_callback = std::move(callback);
        SERVICE_TABLE_ENTRYW dispatchTable[] = {
            { const_cast<wchar_t*>(name), ServiceMain },
            { nullptr, nullptr }
        };
        if (!StartServiceCtrlDispatcherW(dispatchTable)) {
            return 1;
        }
        return 0;
    }

    static void ConfigureRestartActions(const wchar_t* name) {
        wil::unique_schandle scManager(OpenSCManagerW(nullptr, nullptr, SC_MANAGER_CONNECT));
        if (!scManager.is_valid()) {
            return;
        }

        wil::unique_schandle service(
            OpenServiceW(scManager.get(), name, SERVICE_CHANGE_CONFIG));
        if (!service.is_valid()) {
            return;
        }

        SC_ACTION action{};
        action.Type = SC_ACTION_RESTART;
        action.Delay = 0;

        SERVICE_FAILURE_ACTIONSW failure{};
        failure.dwResetPeriod = 600;
        failure.lpRebootMsg = nullptr;
        failure.lpCommand = nullptr;
        failure.cActions = 1;
        failure.lpsaActions = &action;

        ChangeServiceConfig2W(service.get(), SERVICE_CONFIG_FAILURE_ACTIONS, &failure);
    }
};

} // namespace efivar::service
