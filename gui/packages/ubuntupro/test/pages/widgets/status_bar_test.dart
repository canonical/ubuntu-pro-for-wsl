import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/core/agent_connection.dart';
import 'package:ubuntupro/pages/widgets/status_bar.dart';

import 'status_bar_test.mocks.dart';

@GenerateMocks([AgentConnection])
void main() {
  group('display agent status', () {
    testWidgets('by default ON when connected', (tester) async {
      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.disconnected);
      final app = buildApp(conn, const StatusBar());
      await tester.pumpWidget(app);
      final agentStatusButton = find.byIcon(Icons.circle_rounded);
      expect(agentStatusButton, findsOneWidget);
    });
    testWidgets('by default ON when disconnected', (tester) async {
      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.connected);
      final app = buildApp(conn, const StatusBar());
      await tester.pumpWidget(app);
      final agentStatusButton = find.byIcon(Icons.circle_rounded);
      expect(agentStatusButton, findsOneWidget);
    });
    testWidgets('can be hidden', (tester) async {
      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.connected);
      final app = buildApp(conn, const StatusBar(showAgentStatus: false));
      await tester.pumpWidget(app);
      final agentStatusButton = find.byIcon(Icons.circle_rounded);
      expect(agentStatusButton, findsNothing);
    });
  });

  group('agent status click', () {
    testWidgets('disabled when connected', (tester) async {
      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.connected);
      final app = buildApp(conn, const StatusBar());
      await tester.pumpWidget(app);
      final agentStatusButton = find.ancestor(
        of: find.byIcon(Icons.circle_rounded),
        matching: find.byType(IconButton),
      );
      // IconButton doesn't expose the `.enabled` property, so we can't check it.
      await tester.tap(agentStatusButton, warnIfMissed: false);
      verifyNever(conn.restartAgent());
    });
    testWidgets('enabled when disconneted', (tester) async {
      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.disconnected);
      final app = buildApp(conn, const StatusBar());
      await tester.pumpWidget(app);
      final agentStatusButton = find.ancestor(
        of: find.byIcon(Icons.circle_rounded),
        matching: find.byType(IconButton),
      );
      await tester.tap(agentStatusButton, warnIfMissed: false);
      verify(conn.restartAgent()).called(1);
    });
  });

  group('report a bug', () {
    testWidgets('GH issue with template', (tester) async {
      Uri? launchedUri;

      final conn = MockAgentConnection();
      when(conn.state).thenReturn(AgentConnectionState.connected);
      final app = buildApp(
        conn,
        StatusBar(
          launchUrlFn: (uri) async {
            launchedUri = uri;
            return true;
          },
        ),
      );
      await tester.pumpWidget(app);
      final agentStatusButton = find.ancestor(
        of: find.byIcon(StatusBar.bugIcon),
        matching: find.byType(IconButton),
      );
      await tester.tap(agentStatusButton);
      expect(launchedUri, isNotNull);
      expect(launchedUri!.host, 'github.com');
      expect(launchedUri!.path, '/canonical/ubuntu-pro-for-wsl/issues/new');
      expect(launchedUri!.queryParameters['labels'], contains('bug'));
      expect(launchedUri!.queryParameters['template'], 'bug_report.yml');
    });
  });
}

Widget buildApp(AgentConnection conn, Widget status) {
  return ChangeNotifierProvider.value(
    value: conn,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      home: Scaffold(
        body: const Center(child: Text('Test')),
        persistentFooterButtons: [status],
      ),
    ),
  );
}
