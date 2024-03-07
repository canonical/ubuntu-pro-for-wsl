import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/agent_api_client.dart';

import 'package:ubuntupro/core/agent_connection.dart';
import 'package:ubuntupro/core/agent_monitor.dart';

import 'agent_connection_test.mocks.dart';

@GenerateMocks([AgentStartupMonitor, AgentApiClient])
void main() {
  group('starts connected', () {
    final monitor = MockAgentStartupMonitor();
    // No new clients will appear, we already have one.
    when(monitor.addNewClientListener(any)).thenReturn(false);

    test('is connected', () async {
      final client = MockAgentApiClient();
      when(client.onConnectionChanged).thenAnswer((_) => const Stream.empty());
      when(monitor.agentApiClient).thenReturn(client);

      final conn = AgentConnection(monitor);

      expect(conn.isConnected, isTrue);
    });

    test('detects changes', () async {
      final events = Stream.fromIterable(
        [ConnectionEvent.dropped, ConnectionEvent.connected],
      );
      final client = MockAgentApiClient();
      when(client.onConnectionChanged).thenAnswer(
        (_) => events,
      );
      when(monitor.agentApiClient).thenReturn(client);

      final conn = AgentConnection(monitor);
      expect(conn.isConnected, isTrue);

      await events.first;
      expect(conn.isConnected, isFalse);

      await events.last;
      expect(conn.isConnected, isTrue);
    });
  });

  group('starts disconnected', () {
    test('is disconnected', () async {
      final monitor = MockAgentStartupMonitor();
      // We don't have one, thus the callback is accepted.
      when(monitor.addNewClientListener(captureAny)).thenReturn(true);

      final conn = AgentConnection(monitor);

      expect(conn.isConnected, isFalse);
    });

    test('never connects', () async {
      final monitor = MockAgentStartupMonitor();
      // We don't have one, thus the callback is accepted.
      when(monitor.addNewClientListener(captureAny)).thenReturn(true);

      final conn = AgentConnection(monitor);
      expect(conn.isConnected, isFalse);

      // Callback never called, we never got a running API client.
      expect(conn.isConnected, isFalse);
    });

    test('reconnects on request', () async {
      final monitor = MockAgentStartupMonitor();
      // We don't have one, thus the callback is accepted.
      when(monitor.addNewClientListener(captureAny)).thenReturn(true);
      when(monitor.reset()).thenAnswer((_) async {});
      when(monitor.start()).thenAnswer((_) => Stream.value(AgentState.ok));
      final client = MockAgentApiClient();
      when(monitor.agentApiClient).thenReturn(client);
      when(client.onConnectionChanged).thenAnswer((_) => const Stream.empty());

      final conn = AgentConnection(monitor);
      expect(conn.isConnected, isFalse);

      await conn.restartAgent();
      expect(conn.isConnected, isTrue);
    });

    test('connects ok', () async {
      AgentApiCallback? callback;
      final monitor = MockAgentStartupMonitor();
      when(monitor.addNewClientListener(captureAny)).thenAnswer((invocation) {
        /// just set the callback for later invocation.
        callback = invocation.positionalArguments.first;
        return true;
      });

      final conn = AgentConnection(monitor);
      expect(conn.isConnected, isFalse);

      // Eventually we get a running API client.
      final client = MockAgentApiClient();
      when(client.onConnectionChanged).thenAnswer((_) => const Stream.empty());
      callback!(client);

      expect(conn.isConnected, isTrue);
    });
  });
}
