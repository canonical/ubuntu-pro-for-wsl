#pragma once
#include <Windows.h>

#include <string>

namespace up4w {
// An RAII wrapper around the PROCESS_INFORMATION structure to ease preventing
// HANDLE leaks.
struct Process : PROCESS_INFORMATION {
  ~Process() {
    if (hThread != nullptr && hThread != INVALID_HANDLE_VALUE) {
      CloseHandle(hThread);
    }
    if (hProcess != nullptr && hProcess != INVALID_HANDLE_VALUE) {
      CloseHandle(hProcess);
    }
  }
};

// An abstraction on top of the pseudo-console device that prevents leaking
// HANDLEs and makes it easier to start processes under itself.
class PseudoConsole {
  HANDLE hInRead = nullptr;
  HANDLE hInWrite = nullptr;
  HANDLE hOutRead = nullptr;
  HANDLE hOutWrite = nullptr;

  HPCON hDevice;

 public:
  /// Constructs a new pseudo-console with the specified [dimensions].
  explicit PseudoConsole(COORD dimensions);

  HANDLE GetReadHandle() const { return hOutRead; }

  /// Starts a child process under this pseudo-console by running the fully
  /// specified [commandLine].
  Process StartProcess(std::wstring commandLine);

  ~PseudoConsole();
};

}  // namespace up4w
