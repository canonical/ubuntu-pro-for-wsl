import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'p4w_ms_store_platform_interface.dart';

@visibleForTesting
abstract class Methods {
  static const launch = 'LaunchFullTrustProcess';
}

/// An implementation of [P4wMsStorePlatform] that uses method channels.
class MethodChannelP4wMsStore extends P4wMsStorePlatform {
  /// The method channel used to interact with the native platform.
  @visibleForTesting
  final methodChannel = const MethodChannel('com.ubuntu.p4w');

  @override
  Future<void> launchFullTrustProcess([List<String>? args]) {
    if (args == null || args.isEmpty) {
      return methodChannel.invokeMethod<void>(
        Methods.launch,
      );
    }

    return methodChannel.invokeMethod<void>(
      Methods.launch,
      args.join(' '),
    );
  }
}
