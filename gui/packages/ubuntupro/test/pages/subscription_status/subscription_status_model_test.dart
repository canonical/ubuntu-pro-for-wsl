import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';

import 'subscription_status_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('instantiation', () {
    final client = MockAgentApiClient();
    final info = SubscriptionInfo();
    info.productId = 'my prod ID';

    test('immutable is org', () async {
      info.immutable = true;
      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, OrgSubscriptionStatusModel);
    });

    test('none subscribes now', () async {
      info.ensureNone();
      info.immutable = false;
      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, SubscribeNowModel);
    });

    test('unset throws', () async {
      expect(
        () {
          SubscriptionStatusModel(SubscriptionInfo(), client);
        },
        throwsUnimplementedError,
      );
    });
    test('store', () async {
      info.ensureMicrosoftStore();
      info.immutable = false;

      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, StoreSubscriptionStatusModel);
    });

    test('user', () async {
      info.ensureUser();
      info.immutable = false;

      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, UserSubscriptionStatusModel);
    });

    test('organization', () async {
      info.ensureOrganization();
      info.immutable = false;

      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, OrgSubscriptionStatusModel);
    });
  });

  test('ms account link', () {
    const product = 'id';
    final model = StoreSubscriptionStatusModel(product);

    expect(model.uri.pathSegments, contains(product));
  });

  test('manual detach pro', () async {
    String? token;
    final client = MockAgentApiClient();
    when(client.applyProToken(any)).thenAnswer((realInvocation) async {
      token = realInvocation.positionalArguments[0] as String;
    });
    final model = UserSubscriptionStatusModel(client);

    // asserts that detachPro calls applyProToken with an empty String.
    expect(token, isNull);
    await model.detachPro();
    expect(token, isEmpty);
  });

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
