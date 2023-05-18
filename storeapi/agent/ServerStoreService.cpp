#include "ServerStoreService.hpp"

#include <winrt/windows.foundation.collections.h>
#include <winrt/windows.system.h>

namespace StoreApi {

using winrt::Windows::Foundation::IInspectable;
using winrt::Windows::System::KnownUserProperties;
using winrt::Windows::System::User;
using winrt::Windows::System::UserAuthenticationStatus;
using winrt::Windows::System::UserType;

using concurrency::task;

task<UserInfo> UserInfo::Current() {
  auto users = co_await User::FindAllAsync(
      UserType::LocalUser, UserAuthenticationStatus::LocallyAuthenticated);

  auto howManyUsers = users.Size();
  if (howManyUsers < 1) {
    throw Exception("No locally authenticated user could be found.");
  }

  if (howManyUsers > 1) {
    throw Exception(std::format(
        "Expected one but found {} locally authenticated users visible.", howManyUsers));
  }

  IInspectable accountName = co_await users.GetAt(0).GetPropertyAsync(
      KnownUserProperties::AccountName());
  co_return UserInfo{.id = winrt::unbox_value<winrt::hstring>(accountName)};
}

}  // namespace StoreApi
