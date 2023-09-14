#include "p4w_ms_store_plugin_impl.h"

#include <base/DefaultContext.hpp>
#include <base/Exception.hpp>
#include <base/Purchase.hpp>
#include <gui/ClientStoreService.hpp>

#include <cstdint>
#include <exception>

namespace p4w_ms_store {

winrt::fire_and_forget PurchaseSubscription(
    HWND topLevelWindow, std::string productId,
    std::shared_ptr<flutter::MethodResult<flutter::EncodableValue>>
        result) try {
  winrt::apartment_context windowContext{};
  StoreApi::ClientStoreService service{topLevelWindow};
  // Blocks a background thread while retrieving the product.
  co_await winrt::resume_background();
  const auto product = service.FetchAvailableProduct(productId);
  // Resumes in the UI thread to display the native dialog.
  co_await windowContext;
  product.PromptUserForPurchase(
      [result](StoreApi::PurchaseStatus status, int32_t error) {
        if (error < 0) {
          winrt::hresult_error err{error};
          result->Error(channelName, winrt::to_string(err.message()));
          return;
        }

        result->Success(static_cast<int>(status));
      });
} catch (StoreApi::Exception& err) {
  result->Error(channelName, err.what());
} catch (winrt::hresult_error& err) {
  result->Error(channelName, winrt::to_string(err.message()));
} catch (std::exception& err) {
  result->Error(channelName, err.what());
} catch (...) {
  result->Error(channelName, "Unknown exception thrown in the native layer.");
}

}  // namespace p4w_ms_store
