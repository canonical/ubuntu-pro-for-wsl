#include "console.hpp"

#include <memory>
#include <type_traits>

#include "error.hpp"

namespace up4w {
PseudoConsole ::~PseudoConsole() {
  if (hInRead != nullptr && hInRead != INVALID_HANDLE_VALUE) {
    CloseHandle(hInRead);
  }
  if (hInWrite != nullptr && hInWrite != INVALID_HANDLE_VALUE) {
    CloseHandle(hInWrite);
  }
  if (hOutRead != nullptr && hOutRead != INVALID_HANDLE_VALUE) {
    CloseHandle(hOutRead);
  }
  if (hOutWrite != nullptr && hOutWrite != INVALID_HANDLE_VALUE) {
    CloseHandle(hOutWrite);
  }
  if (hDevice != nullptr && hDevice != INVALID_HANDLE_VALUE) {
    ClosePseudoConsole(hDevice);
  }
}

PseudoConsole::PseudoConsole(COORD coordinates) {
  SECURITY_ATTRIBUTES sa{sizeof(SECURITY_ATTRIBUTES), nullptr, true};
  if (!CreatePipe(&hInRead, &hInWrite, &sa, 0)) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }
  const wchar_t pipeName[] = L"\\\\.\\pipe\\UP4WPCon";
  // This handle reads from the child process' stdout.
  hOutRead = CreateNamedPipe(
      pipeName,
      // data flows into this process, reads will be asynchronous.
      PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED,
      // PIPE_WAIT doesn't block with OVERLAPPED IO: see
      // https://devblogs.microsoft.com/oldnewthing/20110114-00/?p=11753
      PIPE_WAIT | PIPE_TYPE_BYTE | PIPE_READMODE_BYTE |
          PIPE_REJECT_REMOTE_CLIENTS,
      1, 0, 0, 0, &sa);
  if (hOutRead == INVALID_HANDLE_VALUE) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }
  // This handle is inherited by the child process' as its stdout.
  // Since we create the handle here, by the time the console creation
  // completes, the pipe is already connected, thus available for an async read
  // operation.
  hOutWrite =
      CreateFile(pipeName, GENERIC_WRITE, 0, NULL, OPEN_EXISTING, 0, NULL);

  if (hOutRead == INVALID_HANDLE_VALUE) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }

  if (auto hr =
          CreatePseudoConsole(coordinates, hInRead, hOutWrite, 0, &hDevice);
      FAILED(hr)) {
    throw hresult_exception{hr};
  }
}

void attr_list_deleter(PPROC_THREAD_ATTRIBUTE_LIST p) {
  if (p) {
    DeleteProcThreadAttributeList(p);
    HeapFree(GetProcessHeap(), 0, p);
  }
};
using unique_attr_list =
    std::unique_ptr<std::remove_pointer_t<PPROC_THREAD_ATTRIBUTE_LIST>,
                    decltype(&attr_list_deleter)>;

/// Returns a list of attributes for process/thread creation with the
/// pseudo-console key enabled and set to [con].
unique_attr_list PseudoConsoleProcessAttrList(HPCON con) {
  PPROC_THREAD_ATTRIBUTE_LIST attrs = nullptr;

  size_t bytesRequired = 0;
  InitializeProcThreadAttributeList(NULL, 1, 0, &bytesRequired);
  // Allocate memory to represent the list
  attrs = static_cast<PPROC_THREAD_ATTRIBUTE_LIST>(
      HeapAlloc(GetProcessHeap(), 0, bytesRequired));
  if (!attrs) {
    throw hresult_exception{E_OUTOFMEMORY};
  }

  // Initialize the list memory location
  if (!InitializeProcThreadAttributeList(attrs, 1, 0, &bytesRequired)) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }

  unique_attr_list result{attrs, &attr_list_deleter};

  if (!UpdateProcThreadAttribute(attrs, 0, PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
                                 con, sizeof(con), NULL, NULL)) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }

  return result;
}

Process PseudoConsole::StartProcess(std::wstring commandLine) {
  unique_attr_list attributes = PseudoConsoleProcessAttrList(hDevice);
  // Prepare Startup Information structure
  STARTUPINFOEX si{};
  si.StartupInfo.cb = sizeof(STARTUPINFOEX);
  si.StartupInfo.hStdInput = hInRead;
  si.StartupInfo.hStdOutput = hOutWrite;
  si.StartupInfo.hStdError = hOutWrite;
  si.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
  si.lpAttributeList = attributes.get();

  Process p{};
  if (!CreateProcessW(NULL, commandLine.data(), NULL, NULL, FALSE,
                      EXTENDED_STARTUPINFO_PRESENT, NULL, NULL, &si.StartupInfo,
                      &p)) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }

  return p;
}

void EventLoop::reserve(std::size_t size) {
  handles_.reserve(size);
  listeners_.reserve(size);
}

EventLoop::EventLoop(
    std::initializer_list<
        std::pair<HANDLE, std::function<std::optional<int>(HANDLE)>>>
        listeners) {
  reserve(listeners.size());
  for (auto& [k, f] : listeners) {
    handles_.push_back(k);
    listeners_.push_back(f);
  }
}

int EventLoop::Run() {
  do {
    DWORD signaledIndex = MsgWaitForMultipleObjectsEx(
        static_cast<DWORD>(handles_.size()), handles_.data(), INFINITE,
        QS_ALLEVENTS, MWMO_INPUTAVAILABLE);
    // none of the handles, thus the window message queue was signaled.
    if (signaledIndex >= handles_.size()) {
      MSG msg;
      if (!GetMessage(&msg, NULL, 0, 0)) {
        // WM_QUIT
        return 0;
      }

      TranslateMessage(&msg);
      DispatchMessage(&msg);
    } else {
      // invoke the listener subscribed to the handle that was signaled.
      if (auto done = listeners_.at(signaledIndex)(handles_.at(signaledIndex));
          done.has_value()) {
        return done.value();
      }
    }
  } while (true);
}

AsyncReader::AsyncReader(HANDLE input) {
  if (input == nullptr || input == INVALID_HANDLE_VALUE) {
    throw std::runtime_error{
        "AsyncReader requires a valid handle but null was passed\n"};
  }
  input_ = input;
  auto event = CreateEvent(nullptr, TRUE, FALSE, nullptr);
  if (event == INVALID_HANDLE_VALUE || event == nullptr) {
    throw hresult_exception(HRESULT_FROM_WIN32(GetLastError()));
  }
  operationState_.hEvent = event;
}

std::string_view up4w::AsyncReader::BytesRead() {
  DWORD read = 0;
  if (FALSE == GetOverlappedResult(input_, &operationState_, &read, FALSE)) {
    throw hresult_exception(HRESULT_FROM_WIN32(GetLastError()));
  }

  // Reset the state.
  bytesRead_ = 0;
  if (FALSE == ResetEvent(operationState_.hEvent)) {
    throw hresult_exception(HRESULT_FROM_WIN32(GetLastError()));
  }

  return std::string_view{buffer_, read};
}

std::optional<int> up4w::AsyncReader::StartRead() {
  // Start an asynchronous read
  auto res =
      ReadFile(input_, buffer_, sizeof(buffer_), &bytesRead_, &operationState_);
  auto lastError = GetLastError();

  // The normal outcome: either the operation fails with ERROR_IO_PENDING or
  // it completes synchronously
  if (res == TRUE || lastError == ERROR_IO_PENDING) {
    return std::nullopt;
  }

  // The writer stopped, not necessarily an error.
  if (lastError == ERROR_BROKEN_PIPE || lastError == ERROR_NO_DATA) {
    return 0;
  }

  // Otherwise, it is an error. Maybe this could even throw.
  return lastError;
}

}  // namespace up4w
