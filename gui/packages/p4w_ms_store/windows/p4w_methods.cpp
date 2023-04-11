#include "p4w_methods.h"

#include <map>
#include <stdexcept>

namespace p4w_ms_store {


static inline const std::map<std::string_view, ChannelUtil::Methods> _methods{
    {"LaunchFullTrustProcess", ChannelUtil::launch},
};

/// Translates the [method_name] to the [Methods] enum for use in switch
/// statements.
///
/// `flutter::MethodCall`s are distinguished by their names (strings). To find
/// the method being requested in C++ we need to `method_name().compare(...`
/// to every candidate. That can be quite annoying. As the number of channels
/// grow this becomes harder to read and even less performant. Using a hash
/// table and an enum allows exhaustive switch statements,
/// `switch (ChannelUtil::method(method_name())) {...}`. That in turn
/// reads much better than a chain of `if(method_name().compare(...)==0)`.
ChannelUtil::Methods ChannelUtil::method(std::string_view method_name) {
  auto found = _methods.find(method_name);
  if (found != _methods.end()) {
    return found->second;
  }
  return Methods::notImplemented;
}

// Allows referring to the method names from the enum. Added mainly for
// testing, thus OK to be O(N).
std::string ChannelUtil::method_name(ChannelUtil::Methods method) {
  if (method == Methods::notImplemented) {
    return "";
  }

  auto view =
      std::find_if(_methods.begin(), _methods.end(),
                   [method](const auto& item) { return item.second == method; })
          ->first;
  return std::string{view.data(), view.length()};
}

// Initializes the underlying variant based on the method_call name.
// It may throw [std::invalid_argument] if the passed arguments don't match
// the underlying method type expectations.
Method::Method(
    const flutter::MethodCall<flutter::EncodableValue>& method_call) {
  switch (ChannelUtil::method(method_call.method_name())) {
    case ChannelUtil::launch:
      _method = LaunchFullTrustProcess(method_call);
      return;

    case ChannelUtil::notImplemented:
      _method = NotImplemented();
      return;
  }
}

/// Initializes the LaunchFullTrustProcess instance.
/// Throws if the arguments are neither a single string nor null.
LaunchFullTrustProcess::LaunchFullTrustProcess(const flutter::MethodCall<flutter::EncodableValue>& method_call)
    : _arguments{std::nullopt} {
  const auto* args = method_call.arguments();
  if (args == nullptr || std::holds_alternative<std::monostate>(*args)) {
    return;
  }
  const auto* cli = std::get_if<std::string>(args);
  // There are arguments, but they are not string.
  if (cli == nullptr) {
    throw std::invalid_argument("LaunchFullTrustProcess requires null or string arguments");
  }

  if (cli->empty()) {
    return;
  }

  _arguments = *cli;
}

// TBC

}  // namespace p4w_ms_store