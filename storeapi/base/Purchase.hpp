#pragma once

/// Types and aliases that must be present in all Context implementations.

#include <cstdint>
#include <functional>

namespace StoreApi {
// We'll certainly want to show in the UI the result of the purchase operation
// in a localizable way. Thus we must agree on the values returned across the
// different languages involved. Since we don't control the Windows Runtime
// APIs, it wouldn't be future-proof to return the raw value of
// StorePurchaseStatus enum right away.
// This must strictly in sync the Dart PurchaseStatus enum in
// https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/gui/packages/p4w_ms_store/lib/p4w_ms_store_platform_interface.dart#L8-L16
// so we don't misinterpret the native call return values.
enum class PurchaseStatus : std::int8_t {
  Succeeded = 0,
  AlreadyPurchased = 1,
  UserGaveUp = 2,
  NetworkError = 3,
  ServerError = 4,
  Unknown = 5,
};

/// A callable the client application must provide to receive the result of the
/// asynchronous purchase operation.
using PurchaseCallback = std::function<void(PurchaseStatus, std::int32_t)>;

}  // namespace StoreApi