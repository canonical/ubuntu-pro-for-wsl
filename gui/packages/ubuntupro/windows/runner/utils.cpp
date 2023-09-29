#include "utils.h"

#include <flutter_windows.h>
#include <io.h>
#include <stdio.h>
#include <windows.h>

#include <array>
#include <iostream>

void CreateAndAttachConsole() {
  if (::AllocConsole()) {
    FILE* unused;
    if (freopen_s(&unused, "CONOUT$", "w", stdout)) {
      _dup2(_fileno(stdout), 1);
    }
    if (freopen_s(&unused, "CONOUT$", "w", stderr)) {
      _dup2(_fileno(stdout), 2);
    }
    std::ios::sync_with_stdio();
    FlutterDesktopResyncOutputStreams();
  }
}

void SetupConsole() {
  // Only succeeds if the parent process is a console app.
  if (::AttachConsole(ATTACH_PARENT_PROCESS)) {
    // In which case we want to know whether the parent is the flutter tool or a
    // CLI shell.
    std::array<wchar_t, 4> flutterSwitchesEnvVar{};
    // What matters is the fact that the env var exists, it's value is
    // irrelevant for this case.
    // https://github.com/flutter/flutter/blob/cfdaf1e593cf0b012bc8ff5a9c1e780ad5fbc153/packages/flutter_tools/lib/src/desktop_device.dart#L217
    if (0 == GetEnvironmentVariableW(
                 L"FLUTTER_ENGINE_SWITCHES", &flutterSwitchesEnvVar[0],
                 // I'm sure the value 4 fits in a DWORD.
                 static_cast<DWORD>(flutterSwitchesEnvVar.size())) &&
        GetLastError() == ERROR_ENVVAR_NOT_FOUND) {
      // Not running by the flutter tool. OK to resync stdio.
      FlutterDesktopResyncOutputStreams();
      // More about the desktop device log reader inside the Flutter tool:
      // https://github.com/flutter/flutter/blob/cfdaf1e593cf0b012bc8ff5a9c1e780ad5fbc153/packages/flutter_tools/lib/src/desktop_device.dart#L310
    }
    // If the parent is not a console app, it could be an IDE, thus check for
    // the presence of a debugger. Otherwise forget about the console.
    // This is a GUI application after all.
  } else {
    if (::IsDebuggerPresent()) {
      CreateAndAttachConsole();
    }
  }
}

std::vector<std::string> GetCommandLineArguments() {
  // Convert the UTF-16 command line arguments to UTF-8 for the Engine to use.
  int argc;
  wchar_t** argv = ::CommandLineToArgvW(::GetCommandLineW(), &argc);
  if (argv == nullptr) {
    return std::vector<std::string>();
  }

  std::vector<std::string> command_line_arguments;

  // Skip the first argument as it's the binary name.
  for (int i = 1; i < argc; i++) {
    command_line_arguments.push_back(Utf8FromUtf16(argv[i]));
  }

  ::LocalFree(argv);

  return command_line_arguments;
}

std::string Utf8FromUtf16(const wchar_t* utf16_string) {
  if (utf16_string == nullptr) {
    return std::string();
  }
  int target_length =
      ::WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, utf16_string, -1,
                            nullptr, 0, nullptr, nullptr);
  std::string utf8_string;
  if (target_length == 0 || target_length > utf8_string.max_size()) {
    return utf8_string;
  }
  utf8_string.resize(target_length);
  int converted_length = ::WideCharToMultiByte(
      CP_UTF8, WC_ERR_INVALID_CHARS, utf16_string, -1, utf8_string.data(),
      target_length, nullptr, nullptr);
  if (converted_length == 0) {
    return std::string();
  }
  return utf8_string;
}
