#include "error.hpp"
#include <chrono>
#include <fstream>

namespace up4w {
std::wstring UnderLocalAppDataPath(std::wstring_view destination) {
  std::wstring_view localAppData = L"LOCALAPPDATA";
  std::wstring resultPath{};
  resultPath.resize(MAX_PATH);

  auto truncatedLength =
      static_cast<DWORD>(resultPath.capacity() - destination.size() - 1);

  auto length =
      GetEnvironmentVariable(localAppData.data(), resultPath.data(), truncatedLength);
  if (length == 0) {
    return {};
  }

  if (length > truncatedLength) {
    throw hresult_exception{CO_E_PATHTOOLONG};
  }

  resultPath.insert(length, destination.data());
  return resultPath;
}

void LogSingleShot(std::filesystem::path const& logFilePath,
                   std::string_view message) {
  auto const time =
      std::chrono::current_zone()->to_local(std::chrono::system_clock::now());

  std::ofstream logfile{logFilePath, std::ios::app};
  logfile << std::format("{:%Y-%m-%d %T}: {}\n", time, message);
}

}  // namespace up4w
