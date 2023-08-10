import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'p4w_ms_store_platform_interface.dart';

/// An implementation of [P4wMsStorePlatform] that uses method channels.
class MethodChannelP4wMsStore extends P4wMsStorePlatform {
  /// The method channel used to interact with the native platform.
  @visibleForTesting
  static const methodChannel = MethodChannel('p4w_ms_store');

  @override
  Future<PurchaseStatus> purchaseSubscription(String productId) async {
    final enumIndex = await methodChannel.invokeMethod<int>(
      'purchaseSubscription', // the method being invoked
      productId, // its arguments.
    );
    // Should never happen.
    if (enumIndex == null || enumIndex < 0) {
      throw PlatformException(code: 'invalid status $enumIndex');
    }
    // Ideally shouldn't happen, but depends on code being in sync.
    if (enumIndex >= PurchaseStatus.values.length) {
      throw PlatformException(code: 'possible mismatched status $enumIndex');
    }

    return PurchaseStatus.values[enumIndex];
  }
}
