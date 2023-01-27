import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

class MockP4wMsStorePlatform
    with MockPlatformInterfaceMixin
    implements P4wMsStorePlatform {

  @override
  Future<String?> getPlatformVersion() => Future.value('42');
}

void main() {
  final P4wMsStorePlatform initialPlatform = P4wMsStorePlatform.instance;

  test('$MethodChannelP4wMsStore is the default instance', () {
    expect(initialPlatform, isInstanceOf<MethodChannelP4wMsStore>());
  });

  test('getPlatformVersion', () async {
    P4wMsStore p4wMsStorePlugin = P4wMsStore();
    MockP4wMsStorePlatform fakePlatform = MockP4wMsStorePlatform();
    P4wMsStorePlatform.instance = fakePlatform;

    expect(await p4wMsStorePlugin.getPlatformVersion(), '42');
  });
}
