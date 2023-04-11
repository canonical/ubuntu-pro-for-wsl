#include "p4w_winrt_api.h"

#include "p4w_channel_constants.h"

#include <flutter/standard_method_codec.h>
#include <windows.h>
#include <winrt/windows.applicationmodel.h>

namespace p4w_ms_store {

namespace am = winrt::Windows::ApplicationModel;

winrt::fire_and_forget WinRtApi::LaunchFullTrustProcess(
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  try {
    co_await am::FullTrustProcessLauncher::
        LaunchFullTrustProcessForCurrentAppAsync();
    result->Success();
  } catch (const winrt::hresult_error& err) {
    std::string msg{winrt::to_string(err.message())};
    result->Error(Constants::ChannelName, msg);
  }
}

winrt::fire_and_forget WinRtApi::LaunchFullTrustProcessWithArgs(
    std::string_view args,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  try {
    auto launch = co_await am::FullTrustProcessLauncher::
        LaunchFullTrustProcessForCurrentAppWithArgumentsAsync(
            winrt::to_hstring(args));
    if (launch.LaunchResult() == am::FullTrustLaunchResult::Success) {
      co_return result->Success();
    }

    winrt::hresult_error err{launch.ExtendedError()};
    std::string msg{winrt::to_string(err.message())};
    result->Error(Constants::ChannelName, msg);
  } catch (const winrt::hresult_error& err) {
    std::string msg{winrt::to_string(err.message())};
    result->Error(Constants::ChannelName, msg);
  }
}

}  // namespace p4w_ms_store