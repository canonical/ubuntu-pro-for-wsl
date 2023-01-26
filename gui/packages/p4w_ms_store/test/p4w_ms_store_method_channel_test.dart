import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';

void main() {
  MethodChannelP4wMsStore platform = MethodChannelP4wMsStore();
  const MethodChannel channel = MethodChannel('p4w_ms_store');

  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    channel.setMockMethodCallHandler((MethodCall methodCall) async {
      return '42';
    });
  });

  tearDown(() {
    channel.setMockMethodCallHandler(null);
  });

  test('getPlatformVersion', () async {
    expect(await platform.getPlatformVersion(), '42');
  });
}
