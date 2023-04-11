/// Calls straight into the Windows Runtime APIs.
/// All precondition logic must be handled out of here.
#ifndef P4W_WINRT_API_H
#define P4W_WINRT_API_H
// Windows
#include <winrt/windows.foundation.h>

// Flutter
#include <flutter/method_channel.h>

// std
#include <memory>

namespace p4w_ms_store {
struct WinRtApi {
  /// Triggers an asynchronous action to launch the full trust process
  /// associated with the current application, sending the result to the Dart
  /// side of the ABI, thus ownership of the [flutter::MethodResult] is
  /// required.
  ///
  /// Since it's a [fire_and_forget] coroutine one doesn't need to co_await on
  /// it. The runtime will take care of the completion.
  static winrt::fire_and_forget LaunchFullTrustProcess(
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

  /// Triggers an asynchronous action to launch the full trust process
  /// associated with the current application passing the command line [args],
  /// and sending the result to the Dart side of the ABI, thus ownership of the
  /// [flutter::MethodResult] is required.
  ///
  /// Since it's a [fire_and_forget] coroutine one doesn't need to co_await on
  /// it. The runtime will take care of the completion.
  static winrt::fire_and_forget LaunchFullTrustProcessWithArgs(
      std::string_view args,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
};
}  // namespace p4w_ms_store
#endif  // P4W_WINRT_API_H