/// Type safe method call implementations.
/// Each class herein defined implements a specific method call.
/// This technique allows for a very simple and stable plugin HandleMethodCall
/// (see p4w_ms_store_plugin.cpp).
/// Each method added to the channel have its type added to the inner [Method]
/// variant.
///
/// Method implementations have:
/// 1. An explicit constructor:
///     `explicit METHOD_NAME(const
///     flutter::MethodCall<flutter::EncodableValue>& method_call);`
/// 2. A template `call` member-function:
///    `template <typename Api> void
///     call(std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>
///     result) const`
/// 3. An entry in the ChannelUtil::Methods scoped enum.
///
/// Constructors must validate the method call arguments, throwing
/// [std::invalid_argument] if validation fails. Most of that validation can
/// be considered defense in depth, since the Dart caller is supposed to
/// prevent misuse of the method channel.
///
/// The `call` member-function is templated on the API for testability.
/// Callers in production should not even notice those are templates.
/// A benefit of using templates is that usually the function calls are inlined.

#ifndef P4W_METHODS_H
#define P4W_METHODS_H

#include <flutter/method_call.h>
#include <flutter/method_result.h>
#include <flutter/standard_message_codec.h>

#include <memory>
#include <optional>
#include <string>
#include <variant>

#include "p4w_winrt_api.h"

namespace p4w_ms_store {

/// Requests the underlying API to launch the full trust process associated with
/// this application. It chooses which API function to call based on whether
/// command line arguments were passed or not.
struct LaunchFullTrustProcess {
  explicit LaunchFullTrustProcess(
      const flutter::MethodCall<flutter::EncodableValue>& method_call);
  template <typename Api>
  void call(std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>
                result) const {
    if (_arguments) {
      Api::LaunchFullTrustProcessWithArgs(_arguments.value(),
                                          std::move(result));
      return;
    }
    Api::LaunchFullTrustProcess(std::move(result));
    return;
  }

 private:
  std::optional<std::string> _arguments;
};

/// The handler for methods not yet implemented. It causes a
/// [MissingPluginException] on the Dart side.
struct NotImplemented {
  template <typename Api>
  void call(std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>
                result) const {
    result->NotImplemented();
  }
};

/// Syntactic suggars to avoid repeating strings all around.
struct ChannelUtil {
  /// Each entry represents a method supported by this plugin.
  enum class Methods : char { launch, notImplemented };  // more to come.

  // This allows for referring to the enum values as `ChannelUtil::launch`
  // instead of `ChannelUtil::Methods::launch`
  using enum Methods;

  /// Enables `switch` on the [method_name].
  static Methods method(std::string_view method_name);

  /// Generates the method name as string from the enum value [method].
  /// Useful for testing.
  static std::string method_name(Methods method);
};

/// All methods supported by this plugin.
using AllMethods =
    std::variant<NotImplemented, LaunchFullTrustProcess>;  // more to come.

/// A façade containing a variant of the supported methods initialized
/// from the flutter::MethodCall.
struct Method {
  AllMethods _method;
  explicit Method(
      const flutter::MethodCall<flutter::EncodableValue>& method_call);

  /// Defers to the underlying variant to handle the call.
  /// Defaults the API type paramenter for clients in production.
  /// See p4w_winrt_api.h.
  template <typename Api = WinRtApi>
  void call(std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>
                result) const {
    std::visit([res = std::move(result)](
                   const auto& m) mutable { m.call<Api>(std::move(res)); },
               _method);
  }
};

}  // namespace p4w_ms_store
#endif  // P4W_METHODS_H