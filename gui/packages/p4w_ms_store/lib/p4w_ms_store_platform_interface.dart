import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'p4w_ms_store_method_channel.dart';

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

  Future<String?> getPlatformVersion() {
    throw UnimplementedError('platformVersion() has not been implemented.');
  }
}
