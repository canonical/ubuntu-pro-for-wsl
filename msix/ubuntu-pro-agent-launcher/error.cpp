#include "error.hpp"
#include <chrono>
#include <fstream>

namespace up4w {
std::wstring MakePathRelativeToEnvDir(std::wstring_view destination,
                                      std::wstring_view envDir) {
  std::wstring resultPath{};
  resultPath.resize(MAX_PATH);

  auto truncatedLength =
      static_cast<DWORD>(resultPath.capacity() - destination.size() - 1);

  auto length =
      GetEnvironmentVariable(envDir.data(), resultPath.data(), truncatedLength);
  if (length == 0) {
    return {};
  }

  resultPath.insert(length, destination.data());
  return resultPath;
}

void LogSingleShot(std::filesystem::path const& logFilePath,
                   std::string_view message) {
  auto const time =
      std::chrono::current_zone()->to_local(std::chrono::system_clock::now());

  std::ofstream logfile{logFilePath};
  logfile << std::format("{:%Y-%m-%d %T}: {}\n", time, message);
}

}  // namespace up4w
