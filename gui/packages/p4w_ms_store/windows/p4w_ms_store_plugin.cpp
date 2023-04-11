#include "p4w_ms_store_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>
#include <windows.h>

#include <memory>
#include <sstream>

#include "p4w_channel_constants.h"
#include "p4w_methods.h"

namespace p4w_ms_store {

// static
void P4wMsStorePlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows* registrar) {
  auto channel =
      std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
          registrar->messenger(), Constants::ChannelName,
          &flutter::StandardMethodCodec::GetInstance());

  auto plugin = std::make_unique<P4wMsStorePlugin>();

  channel->SetMethodCallHandler(
      [plugin_pointer = plugin.get()](const auto& call, auto result) {
        plugin_pointer->HandleMethodCall(call, std::move(result));
      });

  registrar->AddPlugin(std::move(plugin));
}

P4wMsStorePlugin::P4wMsStorePlugin() {}

P4wMsStorePlugin::~P4wMsStorePlugin() {}

void P4wMsStorePlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  try {
    Method m(method_call);
    m.call(std::move(result));
    return;
  } catch (const std::invalid_argument& err) {
    result->Error(Constants::ChannelName, err.what());
  }
}

}  // namespace p4w_ms_store
