import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_client.dart';

import '../utils/mock_grpc.dart';

void main() {
  test('ping fails', timeout: const Timeout(Duration(seconds: 5)), () async {
    final client = AgentApiClient(host: '127.0.0.1', port: 9);
    // IANA discard protol: There should be no service running at this port.
    expect(await client.ping(), isFalse);
  });

  group('with mocked grpc', () {
    final client = AgentApiClient(
      host: '127.0.0.1',
      port: 9,
      stubFactory: MockUIClient.new,
    );

    test('ping succeeds', () async {
      expect(await client.ping(), isTrue);
    });

    test('no subscription info', () async {
      final info = await client.subscriptionInfo();
      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
    });
    test('pro attach user subscription', () async {
      await client.applyProToken('C123');
      final info = await client.subscriptionInfo();

      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.user);
    });

    test('pro detach no subscription again', () async {
      final info = await client.applyProToken('');

      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
    });

    test('setting landscape config succeeds', () async {
      await expectLater(
        () async => await client.applyLandscapeConfig('test config'),
        returnsNormally,
      );
    });

    test('connect to new endpoint', () async {
      final otherRef = client;
      final connected = await otherRef.connectTo(host: 'localhost', port: 9);
      // A real connection would never succeed in port 9 (Discard Protocol).
      expect(connected, isTrue);
      // The client object is still the same, although internals may have changed.
      expect(otherRef.hashCode, client.hashCode);
    });
  });
}
