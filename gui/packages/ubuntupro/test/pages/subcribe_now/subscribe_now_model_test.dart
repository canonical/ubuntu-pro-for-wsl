import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_model.dart';

import 'subscribe_now_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('purchase', () {
    const pluginChannel = MethodChannelP4wMsStore.methodChannel;
    final pluginMessenger =
        TestWidgetsFlutterBinding.ensureInitialized().defaultBinaryMessenger;
    // Resets the plugin message handler after each test.
    tearDown(() {
      pluginMessenger.setMockMethodCallHandler(pluginChannel, null);
    });

    final client = MockAgentApiClient();

    test('disabled by default', () {
      final model = SubscribeNowModel(client);
      expect(model.purchaseAllowed(), isFalse);
    });

    test('expected failure', () async {
      const expectedError = Left(PurchaseStatus.userGaveUp);
      pluginMessenger.setMockMethodCallHandler(pluginChannel, (_) async {
        return expectedError.value.index;
      });
      final model = SubscribeNowModel(client);
      final result = await model.purchaseSubscription();
      expect(result, expectedError);
    });
    test('platform exception', () async {
      const expectedError = Left(PurchaseStatus.unknown);
      pluginMessenger.setMockMethodCallHandler(pluginChannel, (_) async {
        throw PlatformException(code: 'unexpected');
      });
      final model = SubscribeNowModel(client);
      final result = await model.purchaseSubscription();
      expect(result, expectedError);
    });
    test('grpc exception', () async {
      const expectedError = Left(PurchaseStatus.unknown);
      pluginMessenger.setMockMethodCallHandler(pluginChannel, (_) async {
        return PurchaseStatus.succeeded.index;
      });
      when(client.notifyPurchase()).thenThrow(
        const GrpcError.custom(42, 'surprise'),
      );
      final model = SubscribeNowModel(client);
      final result = await model.purchaseSubscription();
      expect(result, expectedError);
    });
    test('success', () async {
      final expectedValue = SubscriptionInfo()..ensureMicrosoftStore();
      pluginMessenger.setMockMethodCallHandler(pluginChannel, (_) async {
        return PurchaseStatus.succeeded.index;
      });
      final client_ = MockAgentApiClient();
      when(client_.notifyPurchase()).thenAnswer(
        (_) async => expectedValue,
      );
      final model = SubscribeNowModel(client_);
      final result = await model.purchaseSubscription();
      expect(result.isRight, isTrue);
      expect(result.getOrThrow(), expectedValue);
    });
  });
}
