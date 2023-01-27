#ifndef FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_
#define FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_

#include <flutter_linux/flutter_linux.h>

G_BEGIN_DECLS

#ifdef FLUTTER_PLUGIN_IMPL
#define FLUTTER_PLUGIN_EXPORT __attribute__((visibility("default")))
#else
#define FLUTTER_PLUGIN_EXPORT
#endif

typedef struct _P4wMsStorePlugin P4wMsStorePlugin;
typedef struct {
  GObjectClass parent_class;
} P4wMsStorePluginClass;

FLUTTER_PLUGIN_EXPORT GType p4w_ms_store_plugin_get_type();

FLUTTER_PLUGIN_EXPORT void p4w_ms_store_plugin_register_with_registrar(
    FlPluginRegistrar* registrar);

G_END_DECLS

#endif  // FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_H_
