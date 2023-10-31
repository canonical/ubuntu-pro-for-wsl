#include "console.hpp"

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

  if (!CreatePipe(&hOutRead, &hOutWrite, &sa, 0)) {
    throw hresult_exception{HRESULT_FROM_WIN32(GetLastError())};
  }

  if (auto hr =
          CreatePseudoConsole(coordinates, hInRead, hOutWrite, 0, &hDevice);
      FAILED(hr)) {
    throw hresult_exception{hr};
  }
}

}  // namespace up4w
