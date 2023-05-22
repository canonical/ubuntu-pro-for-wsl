import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'p4w_ms_store_method_channel.dart';

// This must strictly in sync the C++ StoreApi::PurchaseStatus enum
// https://github.com/canonical/ubuntu-pro-for-windows/blob/main/storeapi/gui/ClientStoreService.hpp#L12-L19
// so we don't misinterpret the native call return values.
enum PurchaseStatus {
  succeeded,
  alreadyPurchased,
  userGaveUp,
  networkError,
  serverError,
  unknown,
}

abstract class P4wMsStorePlatform extends PlatformInterface {
  /// Constructs a P4wMsStorePlatform.
  P4wMsStorePlatform() : super(token: _token);

  static final Object _token = Object();

  static P4wMsStorePlatform _instance = MethodChannelP4wMsStore();

  /// The default instance of [P4wMsStorePlatform] to use.
  ///
  /// Defaults to [MethodChannelP4wMsStore].
  static P4wMsStorePlatform get instance => _instance;

  /// Platform-specific implementations should set this with their own
  /// platform-specific class that extends [P4wMsStorePlatform] when
  /// they register themselves.
  static set instance(P4wMsStorePlatform instance) {
    PlatformInterface.verifyToken(instance, _token);
    _instance = instance;
  }

  Future<PurchaseStatus> purchaseSubscription(String productId) {
    throw UnimplementedError(
      'purchaseSubscription() has not been implemented.',
    );
  }
}
