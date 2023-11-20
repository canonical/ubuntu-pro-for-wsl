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
    final mockGrpc = MockUIClient();
    final client = AgentApiClient.withClient(mockGrpc);

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
  });
}
