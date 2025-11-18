/// A Windows Application that creates an invisible pseudo-console to host the
/// ubuntu-pro-agent.exe;
#include <windows.h>

#include <array>
#include <cstddef>
#include <exception>
#include <format>
#include <optional>

#include "console.hpp"
#include "error.hpp"

std::filesystem::path const& logPath() {
  static std::filesystem::path localAppDataPath = up4w::UnderLocalAppDataPath(
      L"\\Ubuntu Pro\\ubuntu-pro-agent-launcher.log");
  return localAppDataPath;
}

std::filesystem::path thisBinaryDir() {
  wchar_t binaryPath[MAX_PATH];
  DWORD fnLength = GetModuleFileName(nullptr, binaryPath, MAX_PATH);
  if (fnLength == 0) {
    return std::filesystem::path();
  }
  std::filesystem::path exePath{std::wstring_view{binaryPath, fnLength}};
  exePath.remove_filename();

  return exePath;
}

int WINAPI wWinMain(HINSTANCE, HINSTANCE, PWSTR pCmdLine, int) try {
  // Request to restart if closed for installing updates.
  RegisterApplicationRestart(
      pCmdLine, RESTART_NO_CRASH | RESTART_NO_HANG | RESTART_NO_REBOOT);
  // setup the app: pipes and console
  up4w::PseudoConsole console{{.X = 80, .Y = 80}};

  // start the child process
  auto agent = thisBinaryDir() / L"ubuntu-pro-agent.exe";
  // Disable ALPN enforcement for gRPC to avoid issues with Landscape SaaS,
  // which won't have ALPN support in time for the beta.
  if (!SetEnvironmentVariableW(L"GRPC_ENFORCE_ALPN_ENABLED", L"false")) {
    throw up4w::hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }
  auto p = console.StartProcess(std::format(L"{} {}", agent.c_str(), pCmdLine));

  up4w::AsyncReader reader{console.GetReadHandle()};
  reader.StartRead();

  // setup the event loop with listeners.
  up4w::EventLoop ev{{
                         p.hProcess,
                         [](HANDLE process) {
                           DWORD exitCode = 0;
                           GetExitCodeProcess(process, &exitCode);
                           return exitCode;
                         },
                     },
                     {
                         reader.Notifier(),
                         [&reader](HANDLE /*event*/) {
                           auto _ = reader.BytesRead();
                           return reader.StartRead();
                         },
                     }};

  // dispatch the event loop
  return ev.Run();

  // log errors, if any.
} catch (up4w::hresult_exception const& err) {
  std::filesystem::path const& localAppDataPath = logPath();
  if (localAppDataPath.empty()) {
    return 1;
  }

  auto msg = std::format("{}\n\t{}", err.message().c_str(), err.where());
  up4w::LogSingleShot(localAppDataPath, msg);
  return 2;
} catch (std::exception const& err) {
  std::filesystem::path const& localAppDataPath = logPath();
  if (localAppDataPath.empty()) {
    return 1;
  }

  up4w::LogSingleShot(localAppDataPath, err.what());
  return 3;
} catch (...) {
  std::filesystem::path const& localAppDataPath = logPath();
  if (localAppDataPath.empty()) {
    return 1;
  }

  up4w::LogSingleShot(localAppDataPath, "An unknown exception was thrown.\n");
  return 4;
}
