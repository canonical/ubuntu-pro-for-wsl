#include "p4w_ms_store_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <memory>

#include "p4w_ms_store_plugin_impl.h"

namespace p4w_ms_store {

// static
void P4wMsStorePlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows* registrar) {
  auto channel =
      std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
          registrar->messenger(), channelName,
          &flutter::StandardMethodCodec::GetInstance());

  auto plugin = std::make_unique<P4wMsStorePlugin>(
      [registrar] { return GetRootWindow(registrar->GetView()); });

  channel->SetMethodCallHandler(
      [plugin_pointer = plugin.get()](const auto& call, auto result) {
        plugin_pointer->HandleMethodCall(call, std::move(result));
      });

  registrar->AddPlugin(std::move(plugin));
}

P4wMsStorePlugin::P4wMsStorePlugin(std::function<HWND()> windowProvider)
    : getRootWindow{std::move(windowProvider)} {}

P4wMsStorePlugin::~P4wMsStorePlugin() {}

void P4wMsStorePlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  if (method_call.method_name().compare("purchaseSubscription") == 0) {
    // std::get_if can handle a nullptr argument.
    auto* product = std::get_if<std::string>(method_call.arguments());
    if (product == nullptr) {
      result->Error(channelName, "A <productId> string argument was expected");
    }

    // When HandleMethodCall gets called we already have a running app, thus a
    // top level window.
    HWND topLevel = getRootWindow();
    PurchaseSubscription(topLevel, *product, std::move(result));
    return;
  } else {
    result->NotImplemented();
  }
}

}  // namespace p4w_ms_store
