import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';

void main() {
  final platform = MethodChannelP4wMsStore();
  const channel = MethodChannelP4wMsStore.methodChannel;
  const productId = 'awesome-addon';

  final binding = TestWidgetsFlutterBinding.ensureInitialized();

  tearDown(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, null);
  });

  test('purchaseSubscription success', () async {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
      methodCall,
    ) async {
      return PurchaseStatus.succeeded.index;
    });
    expect(
      await platform.purchaseSubscription(productId),
      PurchaseStatus.succeeded,
    );
  });
  test('purchaseSubscription throws on invalid response', () async {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
      methodCall,
    ) async {
      return -10;
    });
    await expectLater(
      platform.purchaseSubscription(productId),
      throwsA(isA<PlatformException>()),
    );
  });

  test('purchaseSubscription throws on null response', () async {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
      methodCall,
    ) async {
      return null;
    });
    await expectLater(
      platform.purchaseSubscription(productId),
      throwsA(isA<PlatformException>()),
    );
  });

  test('purchaseSubscription throws out of sync response', () async {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
      methodCall,
    ) async {
      return PurchaseStatus.values.length;
    });
    await expectLater(
      platform.purchaseSubscription(productId),
      throwsA(isA<PlatformException>()),
    );
  });
}
