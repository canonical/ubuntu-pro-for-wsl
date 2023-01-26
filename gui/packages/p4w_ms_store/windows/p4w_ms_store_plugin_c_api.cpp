#include "include/p4w_ms_store/p4w_ms_store_plugin_c_api.h"

#include <flutter/plugin_registrar_windows.h>

#include "p4w_ms_store_plugin.h"

void P4wMsStorePluginCApiRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar) {
  p4w_ms_store::P4wMsStorePlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}
