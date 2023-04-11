import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

void main() {
  final initialPlatform = P4wMsStorePlatform.instance;

  test('$MethodChannelP4wMsStore is the default instance', () {
    expect(initialPlatform, isInstanceOf<MethodChannelP4wMsStore>());
  });

  group('launchFullTrustProcess', () {
    test('completes ok', () async {
      final p4wMsStorePlugin = P4wMsStore();
      final fakePlatform = OkP4wMsStorePlatform();
      P4wMsStorePlatform.instance = fakePlatform;

      expect(p4wMsStorePlugin.launchFullTrustProcess(), completes);
    });

    test('expected failure', () async {
      final p4wMsStorePlugin = P4wMsStore();
      final fakePlatform = FailingP4wMsStorePlatform();
      P4wMsStorePlatform.instance = fakePlatform;

      await expectLater(
        p4wMsStorePlugin.launchFullTrustProcess(),
        throwsA(isA<PlatformException>()),
      );
    });
  });
}

class OkP4wMsStorePlatform
    with MockPlatformInterfaceMixin
    implements P4wMsStorePlatform {
  @override
  Future<void> launchFullTrustProcess([List<String>? args]) async {}
}

class FailingP4wMsStorePlatform
    with MockPlatformInterfaceMixin
    implements P4wMsStorePlatform {
  @override
  Future<void> launchFullTrustProcess([List<String>? args]) async =>
      throw PlatformException(code: 'test');
}
