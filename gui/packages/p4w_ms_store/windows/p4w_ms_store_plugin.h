#ifndef FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_
#define FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>

#include <memory>

namespace p4w_ms_store {

class P4wMsStorePlugin : public flutter::Plugin {
 public:
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows *registrar);

  P4wMsStorePlugin();

  virtual ~P4wMsStorePlugin();

  // Disallow copy and assign.
  P4wMsStorePlugin(const P4wMsStorePlugin&) = delete;
  P4wMsStorePlugin& operator=(const P4wMsStorePlugin&) = delete;

 private:
  // Called when a method is called on this plugin's channel from Dart.
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue> &method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
};

}  // namespace p4w_ms_store

#endif  // FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_
