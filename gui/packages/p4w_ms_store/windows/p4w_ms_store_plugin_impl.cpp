#include "p4w_ms_store_plugin_impl.h"

#include <flutter/encodable_value.h>

#include <gui/ClientStoreService.hpp>

namespace p4w_ms_store {

winrt::fire_and_forget PurchaseSubscription(
    HWND topLevelWindow, std::string productId,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  try {
    StoreApi::ClientStoreService service{topLevelWindow};
    const auto res =
        co_await service.PromptUserToSubscribe(std::move(productId));
    result->Success(static_cast<int>(res));
  } catch (StoreApi::Exception& err) {
    result->Error(channelName, err.what());
  } catch (winrt::hresult_error& err) {
    result->Error(channelName, winrt::to_string(err.message()));
  } catch (std::exception& err) {
    result->Error(channelName, err.what());
  }
}

}  // namespace p4w_ms_store
