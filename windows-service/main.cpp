#include "app.hpp"
#include "service_control.hpp"

#include <windows.h>

namespace {

efivar::service::ServiceApp* g_app = nullptr;

BOOL WINAPI ConsoleCtrlHandler(DWORD ctrlType) noexcept {
    if (ctrlType == CTRL_C_EVENT || ctrlType == CTRL_BREAK_EVENT ||
        ctrlType == CTRL_CLOSE_EVENT || ctrlType == CTRL_SHUTDOWN_EVENT) {
        if (g_app) {
            SetEvent(g_app->stop_event());
        }
        return TRUE;
    }
    return FALSE;
}

} // namespace

int wmain(int argc, wchar_t* argv[]) {
    bool runAsService = false;
    for (int i = 1; i < argc; ++i) {
        if (std::wcscmp(argv[i], L"service") == 0) {
            runAsService = true;
            break;
        }
    }

    if (runAsService) {
        return efivar::service::ServiceControl::Run(
            L"EfivarService",
            [](HANDLE stopEvent) {
                auto app = efivar::service::ServiceApp::Initialize(
                    efivar::service::Mode::Service, stopEvent);
                if (!app) {
                    return;
                }
                app->Run();
            });
    }

    auto app = efivar::service::ServiceApp::Initialize(efivar::service::Mode::Console);
    if (!app) {
        return 1;
    }
    g_app = &*app;
    SetConsoleCtrlHandler(ConsoleCtrlHandler, TRUE);
    app->Run();
    g_app = nullptr;
    return 0;
}
