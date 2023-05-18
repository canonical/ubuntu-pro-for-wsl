#include "ServerStoreService.hpp"

#include <winrt/windows.foundation.collections.h>
#include <winrt/windows.security.cryptography.core.h>
#include <winrt/windows.security.cryptography.h>
#include <winrt/windows.system.h>

namespace StoreApi {

using winrt::Windows::Foundation::IInspectable;
using winrt::Windows::Security::Cryptography::BinaryStringEncoding;
using winrt::Windows::Security::Cryptography::CryptographicBuffer;
using winrt::Windows::Security::Cryptography::Core::HashAlgorithmNames;
using winrt::Windows::Security::Cryptography::Core::HashAlgorithmProvider;
using winrt::Windows::System::KnownUserProperties;
using winrt::Windows::System::User;
using winrt::Windows::System::UserAuthenticationStatus;
using winrt::Windows::System::UserType;

using concurrency::task;

winrt::hstring sha256(winrt::hstring input) {
  auto inputUtf8 = CryptographicBuffer::ConvertStringToBinary(
      input, BinaryStringEncoding::Utf8);
  auto hasher =
      HashAlgorithmProvider::OpenAlgorithm(HashAlgorithmNames::Sha256());
  return CryptographicBuffer::EncodeToHexString(hasher.HashData(inputUtf8));
}

task<UserInfo> UserInfo::Current() {
  // The preferred strategy (defined in the spec) is a plain SHA256 hash of the
  // user email acquired from the runtime API, which provides the AccountName
  // property of the current user.
  auto users = co_await User::FindAllAsync(
      UserType::LocalUser, UserAuthenticationStatus::LocallyAuthenticated);

  auto howManyUsers = users.Size();
  if (howManyUsers < 1) {
    throw Exception("No locally authenticated user could be found.");
  }

  if (howManyUsers > 1) {
    throw Exception(std::format(
        "Expected one but found {} locally authenticated users visible.",
        howManyUsers));
  }

  IInspectable accountName = co_await users.GetAt(0).GetPropertyAsync(
      KnownUserProperties::AccountName());
  auto name = winrt::unbox_value<winrt::hstring>(accountName);
  if (name.size() == 0) {
    co_return {};
  }

  co_return UserInfo{.id = sha256(name)};
}

}  // namespace StoreApi
