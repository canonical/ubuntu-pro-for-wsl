import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/startup/agent_monitor.dart';
import 'package:ubuntupro/pages/startup/startup_model.dart';
import 'package:ubuntupro/pages/startup/startup_page.dart';

import 'startup_page_test.mocks.dart';

const lastText = 'LAST TEXT';
MaterialApp buildApp(StartupModel model) => MaterialApp(
      home: ChangeNotifierProvider.value(
        value: model,
        child: const StartupAnimatedChild(nextRoute: '/next'),
      ),
      routes: {'/next': (_) => const Text(lastText)},
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    );

@GenerateMocks([AgentStartupMonitor, AgentApiClient])
void main() {
  testWidgets('starts in progres', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable(
        [
          AgentState.querying,
        ],
      ),
    );
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    expect(find.byType(LinearProgressIndicator), findsOneWidget);
  });

  testWidgets('navigates when model is ok', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable(
        [
          AgentState.querying,
          AgentState.ok,
        ],
      ),
    );
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    await model.init();
    await tester.pumpAndSettle();

    expect(find.byType(LinearProgressIndicator), findsNothing);
    expect(find.text(lastText), findsOneWidget);
  });

  testWidgets('button for retry', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable(
        [
          AgentState.querying,
          AgentState.starting,
          AgentState.invalid,
          AgentState.unreachable,
        ],
      ),
    );
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    await model.init();
    await tester.pumpAndSettle();

    // no success
    expect(find.text(lastText), findsNothing);
    // show error icon
    expect(find.byIcon(Icons.error_outline), findsOneWidget);
    // and a retry button
    expect(find.byType(OutlinedButton), findsOneWidget);
  });

  testWidgets('terminal error no button', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable(
        [
          AgentState.querying,
          AgentState.starting,
          AgentState.cannotStart,
        ],
      ),
    );
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    await model.init();
    await tester.pumpAndSettle();

    // no success
    expect(find.text(lastText), findsNothing);
    // show error icon
    expect(find.byIcon(Icons.error_outline), findsOneWidget);
    // but no retry button
    expect(find.byType(OutlinedButton), findsNothing);
  });

  testWidgets('creates a model', (tester) async {
    final mockClient = MockAgentApiClient();
    // Fakes a successful ping.
    when(mockClient.ping()).thenAnswer((_) async => true);
    // Builds a less trivial app using the higher level Startup widget
    // to evaluate whether the instantiation of the model happens.
    final app = MaterialApp(
      home: Provider<AgentStartupMonitor>(
        create: (context) => AgentStartupMonitor(
          appName: 'app name',
          addrFileName: 'anywhere',
          agentLauncher: () async => true,
          clientFactory: (port) =>
              AgentApiClient(host: 'localhost', port: port),
          onClient: (_) {},
        ),
        child: const StartupPage(
          nextRoute: '/next',
        ),
      ),
      routes: {'/next': (_) => const Text(lastText)},
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    );

    await tester.pumpWidget(app);
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(StartupAnimatedChild));
    final model = Provider.of<StartupModel>(context, listen: false);

    expect(model, isNotNull);
  });
}
