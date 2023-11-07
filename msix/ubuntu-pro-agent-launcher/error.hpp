#pragma once
#include <Windows.h>

#include <filesystem>
#include <memory>
#include <source_location>
#include <string>

namespace up4w {
// A small RAII wrapper around Win32 heap allocated strings.
class unique_string {
  static void string_buffer_deleter(char* buffer) {
    if (buffer) {
      HeapFree(GetProcessHeap(), 0, buffer);
    }
  }
  using unique_str = std::unique_ptr<char, decltype(&string_buffer_deleter)>;
  unique_str buffer_;

 public:
  const char* c_str() { return buffer_.get(); }
  explicit unique_string(char* buffer)
      : buffer_{buffer, &string_buffer_deleter} {}
};

/// Wraps Windows HRESULT into something that resembles a std::exception
class hresult_exception {
  HRESULT value;
  std::source_location loc_;

 public:
  unique_string message() const {
    char* buffer = nullptr;
    ::FormatMessageA(
        FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_ALLOCATE_BUFFER, nullptr,
        // Due the FORMAT_MESSAGE_ALLOCATE_BUFFER flag, what should be a char*
        // (the buffer variable) has to be treated as char**, even though it's
        // passed as char*. See
        // https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-formatmessagea
        value, 0, (LPSTR)&buffer, 0, nullptr);
    return unique_string{buffer};
  }
  std::string where() const {
    return std::format("{}: {} ({})", loc_.file_name(), loc_.line(),
                       loc_.function_name());
  }

  explicit hresult_exception(HRESULT value, std::source_location location =
                                                std::source_location::current())
      : value(value), loc_{location} {}
  hresult_exception(const hresult_exception& other) = default;
  hresult_exception(hresult_exception&& other) noexcept = default;
  ~hresult_exception() = default;
};

/// Computes the absolute path resulting of joining the [destination] into the
/// value of the environment variable [LOCALAPPDATA]. Returns empty string if the
/// environment variable is undefined.
std::wstring UnderLocalAppDataPath(std::wstring_view destination);

// Opens the log file, writes the message with a timestamp and closes it.
void LogSingleShot(std::filesystem::path const& logFilePath,
                   std::string_view message);

}  // namespace up4w
