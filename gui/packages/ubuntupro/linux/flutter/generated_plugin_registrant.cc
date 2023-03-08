//
//  Generated file. Do not edit.
//

// clang-format off

#include "generated_plugin_registrant.h"

#include <p4w_ms_store/p4w_ms_store_plugin.h>

void fl_register_plugins(FlPluginRegistry* registry) {
  g_autoptr(FlPluginRegistrar) p4w_ms_store_registrar =
      fl_plugin_registry_get_registrar_for_plugin(registry, "P4wMsStorePlugin");
  p4w_ms_store_plugin_register_with_registrar(p4w_ms_store_registrar);
}
