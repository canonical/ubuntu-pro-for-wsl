import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart' as grpc;
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
      final src = await client.configSources();
      final info = src.proSubscription;
      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
      expect(
        src.landscapeSource.whichLandscapeSourceType(),
        LandscapeSourceType.none,
      );
    });
    test('pro attach user subscription', () async {
      await client.applyProToken('C123');
      final src = await client.configSources();
      final info = src.proSubscription;

      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.user);
    });

    test('pro detach no subscription again', () async {
      final info = await client.applyProToken('');

      expect(info.productId, isEmpty);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
    });

    test('setting landscape config succeeds', () async {
      await client.applyLandscapeConfig('test config');
      final src = await client.configSources();
      expect(
        src.landscapeSource.whichLandscapeSourceType(),
        LandscapeSourceType.user,
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

  group('connection events', () {
    test('idle is ok', () {
      // assuming we start in the grpc.ConnectionState.ready state.
      final grpcEvents = Stream.fromIterable([
        grpc.ConnectionState.idle,
        grpc.ConnectionState.connecting,
        grpc.ConnectionState.ready,
      ]);

      expect(
        mapGRPCConnectionEvents(grpcEvents),
        emitsInOrder(<ConnectionEvent>[
          ConnectionEvent.connected,
          ConnectionEvent.dropped,
          ConnectionEvent.connected,
        ]),
      );
    });
    test('conn dropped', () {
      // assuming we start in the grpc.ConnectionState.ready state.
      final grpcEvents = Stream.fromIterable([
        grpc.ConnectionState.connecting,
        grpc.ConnectionState.transientFailure,
        grpc.ConnectionState.connecting,
        grpc.ConnectionState.shutdown,
      ]);

      expect(
        mapGRPCConnectionEvents(grpcEvents),
        emitsInOrder(<ConnectionEvent>[
          ConnectionEvent.dropped,
          ConnectionEvent.dropped,
          ConnectionEvent.dropped,
          ConnectionEvent.dropped,
        ]),
      );
    });
  });
}
