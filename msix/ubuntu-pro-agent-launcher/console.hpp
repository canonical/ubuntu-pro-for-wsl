#pragma once
#include <Windows.h>

#include <functional>
#include <initializer_list>
#include <optional>
#include <string>
#include <type_traits>
#include <utility>
#include <vector>

namespace up4w {
// An RAII wrapper around the PROCESS_INFORMATION structure to ease preventing
// HANDLE leaks.
struct Process : PROCESS_INFORMATION {
  Process(Process const& other) = delete;
  Process& operator=(Process const& other) = delete;
  Process(Process&& other) noexcept { *this = std::move(other); }
  Process& operator=(Process&& other) noexcept {
    hProcess = std::exchange(other.hProcess, nullptr);
    hThread = std::exchange(other.hThread, nullptr);
    dwProcessId = std::exchange(other.dwProcessId, 0);
    dwThreadId = std::exchange(other.dwThreadId, 0);
    return *this;
  }
  Process() noexcept {
    hProcess = nullptr;
    hThread = nullptr;
    dwProcessId = 0;
    dwThreadId = 0;
  }

  ~Process() noexcept {
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
  /// specified [commandLine]. The child process inherits the parent environment.
  Process StartProcess(std::wstring commandLine);

  ~PseudoConsole();
};

/// A combination of traditional window message loop with event listening.
/// Listener functions return any integer value other than nullopt to report
/// that the event loop should exit.
class EventLoop {
  std::vector<HANDLE> handles_;
  std::vector<std::function<std::optional<int>(HANDLE)>> listeners_;
  void reserve(std::size_t size);

 public:
  explicit EventLoop(
      std::initializer_list<
          std::pair<HANDLE, std::function<std::optional<int>(HANDLE)>>>
          listeners);

  // Runs the event loop until one of the listeners return a value or a closing
  // message is received in the message queue.
  int Run();
};

/// A helper class for consistent asynchronously reading an [input] HANDLE.
class AsyncReader {
  // The input this will read from.
  HANDLE input_ = nullptr;
  // The asynchronous operation state.
  OVERLAPPED operationState_ = {0};
  // A buffer to hold the contents of the last successful read.
  char buffer_[2048] = {0};
  DWORD bytesRead_ = 0;

 public:
  // Creates a new AsyncReader storing the [input] handle to read from.
  explicit AsyncReader(HANDLE input);
  // The handle one must watch for to be notified when the in-flight async read
  // operation completes.
  HANDLE Notifier() const { return operationState_.hEvent; }

  // Starts an asynchronous read operation from [input_]. Upon completion, a
  // view of the result can be acquired by calling [BytesRead()]. An optional
  // error code is returned in case the operation fails to start.
  std::optional<int> StartRead();
  std::string_view BytesRead();

  ~AsyncReader() {
    if (operationState_.hEvent) {
      CloseHandle(operationState_.hEvent);
    }
  }
};

}  // namespace up4w
