import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'p4w_ms_store_platform_interface.dart';

/// An implementation of [P4wMsStorePlatform] that uses method channels.
class MethodChannelP4wMsStore extends P4wMsStorePlatform {
  /// The method channel used to interact with the native platform.
  @visibleForTesting
  final methodChannel = const MethodChannel('p4w_ms_store');

  @override
  Future<String?> getPlatformVersion() async {
    final version =
        await methodChannel.invokeMethod<String>('getPlatformVersion');
    return version;
  }
}
