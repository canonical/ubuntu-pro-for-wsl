import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';

void main() {
  final platform = MethodChannelP4wMsStore();
  const channel = MethodChannel('p4w_ms_store');
  const productId = 'awesome-addon';

  final binding = TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel,
        (methodCall) async {
      return PurchaseStatus.succeeded.index;
    });
  });

  tearDown(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, null);
  });

  test('purchaseSubscription', () async {
    expect(
      await platform.purchaseSubscription(productId),
      PurchaseStatus.succeeded,
    );
  });
}
