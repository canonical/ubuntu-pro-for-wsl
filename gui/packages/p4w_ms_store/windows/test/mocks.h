#include <functional>
#include <memory>

#include "p4w_channel_constants.h"
#include "p4w_winrt_api.h"

// A stub allowing customization on the call site. By default its methods do
// nothing. Requires all tests using it to be synchronous.
struct StubApi {
  static inline std::unique_ptr<StubApi> instance{nullptr};

  std::function<winrt::fire_and_forget(
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>)>
      on_launch;
  std::function<winrt::fire_and_forget(
      std::string_view,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>)>
      on_launch_with_args;

  static winrt::fire_and_forget LaunchFullTrustProcess(
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    if (instance->on_launch) {
      return instance->on_launch(std::move(result));
    }
    return {};
  }

  static winrt::fire_and_forget LaunchFullTrustProcessWithArgs(
      std::string_view args,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    if (instance->on_launch_with_args) {
      return instance->on_launch_with_args(args, std::move(result));
    }
    return {};
  }
};