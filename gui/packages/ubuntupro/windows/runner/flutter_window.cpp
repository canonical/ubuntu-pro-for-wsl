#include "flutter_window.h"

#include <flutter/standard_method_codec.h>

#include <optional>
#include <type_traits>

#include "flutter/generated_plugin_registrant.h"

FlutterWindow::FlutterWindow(const flutter::DartProject& project)
    : project_(project) {}

FlutterWindow::~FlutterWindow() {}

bool FlutterWindow::OnCreate() {
  SetWindowText(GetHandle(), L"Ubuntu Pro");
  if (!Win32Window::OnCreate()) {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(
      frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view()) {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());
  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  flutter_controller_->engine()->SetNextFrameCallback([&]() { this->Show(); });

  integrationTestChannel = std::make_unique<
      flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(),
      // https://github.com/flutter/flutter/blob/master/packages/integration_test/lib/src/channel.dart#L9
      "plugins.flutter.io/integration_test",
      &flutter::StandardMethodCodec::GetInstance());

  integrationTestChannel->SetMethodCallHandler(
      [this](auto const& call, auto result) {
        HandleMethodCall(call, std::move(result));
      });

  return true;
}

void FlutterWindow::HandleMethodCall(
    flutter::MethodCall<flutter::EncodableValue> const& call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  // https://github.com/flutter/flutter/blob/master/packages/integration_test/lib/integration_test.dart#L55-L63
  if (call.method_name().compare("allTestsFinished") == 0) {
    result->Success();
    ::PostMessage(GetHandle(), WM_CLOSE, 0, 0);
    return;
  }
}

void FlutterWindow::OnDestroy() {
  if (flutter_controller_) {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message,
                              WPARAM const wparam,
                              LPARAM const lparam) noexcept {
  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_) {
    std::optional<LRESULT> result =
        flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam,
                                                      lparam);
    if (result) {
      return *result;
    }
  }

  switch (message) {
    case WM_FONTCHANGE:
      flutter_controller_->engine()->ReloadSystemFonts();
      break;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
