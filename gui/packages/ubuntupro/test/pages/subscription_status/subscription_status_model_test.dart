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
    final landscape = LandscapeSource();
    info.productId = 'my prod ID';

    test('no subscription throws', () async {
      info.ensureNone();
      expect(() {
        SubscriptionStatusModel(
          ConfigSources(proSubscription: info, landscapeSource: landscape),
          client,
        );
      }, throwsUnimplementedError);
    });

    test('unset throws', () async {
      expect(() {
        SubscriptionStatusModel(ConfigSources(), client);
      }, throwsUnimplementedError);
    });
    test('store', () async {
      info.ensureMicrosoftStore();

      final model = SubscriptionStatusModel(
        ConfigSources(proSubscription: info, landscapeSource: landscape),
        client,
      );
      expect(model.runtimeType, StoreSubscriptionStatusModel);
    });

    test('user', () async {
      info.ensureUser();

      final model = SubscriptionStatusModel(
        ConfigSources(proSubscription: info, landscapeSource: landscape),
        client,
      );
      expect(model.runtimeType, UserSubscriptionStatusModel);
    });

    test('organization', () async {
      info.ensureOrganization();

      final model = SubscriptionStatusModel(
        ConfigSources(proSubscription: info, landscapeSource: landscape),
        client,
      );
      expect(model.runtimeType, OrgSubscriptionStatusModel);
    });
  });

  group('config Landscape:', () {
    final client = MockAgentApiClient();
    final subscriptions = [
      SubscriptionInfo()..ensureOrganization(),
      SubscriptionInfo()..ensureMicrosoftStore(),
      SubscriptionInfo()..ensureUser(),
    ];
    final landscapeSources = [
      LandscapeSource()..ensureNone(),
      LandscapeSource()..ensureOrganization(),
      LandscapeSource()..ensureUser(),
    ];

    String makeSubTestName(
      LandscapeSource landscape,
      SubscriptionInfo sub,
      bool globallyEnabled,
    ) {
      if (!globallyEnabled) {
        return 'landscape ${landscape.toString().split(':').first} with pro ${sub.toString().split(':').first} => disallowed globally';
      }

      final want = landscape.hasOrganization() ? 'disallowed' : 'allowed';
      return 'landscape ${landscape.toString().split(':').first} with pro ${sub.toString().split(':').first} => $want';
    }

    for (final enabled in [true, false]) {
      for (final subs in subscriptions) {
        for (final landscape in landscapeSources) {
          test(makeSubTestName(landscape, subs, enabled), () {
            final want =
                enabled && !landscape.hasOrganization() ? isTrue : isFalse;

            final model = SubscriptionStatusModel(
              ConfigSources(proSubscription: subs, landscapeSource: landscape),
              client,
              canConfigureLandscape: enabled,
            );
            expect(model.canConfigureLandscape, want);
          });
        }
      }
    }
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
      if (token != null && token!.isNotEmpty) {
        return SubscriptionInfo()..ensureUser();
      }
      return SubscriptionInfo()..ensureNone();
    });
    final model = UserSubscriptionStatusModel(client);

    // asserts that detachPro calls applyProToken with an empty String.
    expect(token, isNull);
    await model.detachPro();
    expect(token, isEmpty);
  });
}
