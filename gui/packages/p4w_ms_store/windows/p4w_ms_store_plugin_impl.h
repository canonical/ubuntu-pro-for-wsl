#ifndef FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_IMPL_H_
#define FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_IMPL_H_

#include <flutter/flutter_view.h>

namespace p4w_ms_store {

// Auxiliary functions and implementation details for the plugin.

// The method channel name as a constant. It'll be referred in different places.
static const char* channelName = "p4w_ms_store";

// Returns the window's HWND for a given FlutterView.
inline HWND GetRootWindow(flutter::FlutterView* view) {
  return ::GetAncestor(view->GetNativeWindow(), GA_ROOT);
}

}  // namespace p4w_ms_store

#endif  // FLUTTER_PLUGIN_P4W_MS_STORE_PLUGIN_IMPL_H_
