import 'package:agentapi/agentapi.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';

import 'subscription_status_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('instantiation', () {
    final client = MockAgentApiClient();
    final info = SubscriptionInfo();
    info.productId = 'my prod ID';

    test('unset throws', () async {
      expect(
        () {
          SubscriptionStatusModel(info, client);
        },
        throwsUnimplementedError,
      );
    });

    test('none throws', () async {
      info.ensureNone();
      expect(
        () {
          SubscriptionStatusModel(info, client);
        },
        throwsUnimplementedError,
      );
    });
    test('store', () async {
      info.ensureMicrosoftStore();

      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, StoreSubscriptionStatusModel);
    });

    test('manual', () async {
      info.ensureManual();

      final model = SubscriptionStatusModel(info, client);
      expect(model.runtimeType, ManualSubscriptionStatusModel);
    });

    test('organization', () async {
      info.ensureOrganization();

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
    final model = ManualSubscriptionStatusModel(client);

    // asserts that detachPro calls applyProToken with an empty String.
    expect(token, isNull);
    await model.detachPro();
    expect(token, isEmpty);
  });
}
