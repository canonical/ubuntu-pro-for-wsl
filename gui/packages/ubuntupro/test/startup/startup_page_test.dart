import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/agent_monitor.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:ubuntupro/pages/startup/startup_model.dart';
import 'package:ubuntupro/pages/startup/startup_page.dart';
import 'package:wizard_router/wizard_router.dart';

import '../utils/build_multiprovider_app.dart';
import 'startup_page_test.mocks.dart';

@GenerateMocks([AgentStartupMonitor, AgentApiClient])
void main() {
  testWidgets('starts in progres', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(
      monitor.start(),
    ).thenAnswer((_) => Stream.fromIterable([AgentState.querying]));
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    expect(find.byType(LinearProgressIndicator), findsOneWidget);
  });

  testWidgets('agent state enum l10n', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(
      monitor.start(),
    ).thenAnswer((_) => Stream.fromIterable([AgentState.querying]));
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));
    final context = tester.element(find.byType(StartupAnimatedChild));
    final lang = AppLocalizations.of(context);
    for (final value in AgentState.values) {
      // localize will throw if new values were added to the enum but not to the method.
      expect(() => value.localize(lang), returnsNormally);
    }
  });

  testWidgets('navigates when model is ok', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable([AgentState.querying, AgentState.ok]),
    );
    final model = StartupModel(monitor);
    await tester.pumpWidget(buildApp(model));

    await tester.pumpAndSettle();

    expect(find.byType(LinearProgressIndicator), findsNothing);
    expect(find.text(lastText), findsOneWidget);
  });

  testWidgets('terminal error no button', (tester) async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable([
        AgentState.querying,
        AgentState.starting,
        AgentState.cannotStart,
      ]),
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
    final app = buildMultiProviderWizardApp(
      providers: [
        Provider<AgentStartupMonitor>(
          create: (context) => AgentStartupMonitor(
            addrFileName: 'anywhere',
            agentLauncher: () async => true,
            clientFactory: AgentApiClient.new,
            onClient: (_) {},
          ),
        ),
      ],
      routes: {
        '/': WizardRoute(builder: (_) => const StartupPage()),
        '/next': WizardRoute(builder: (_) => const Text(lastText)),
      },
    );

    await tester.pumpWidget(app);

    final context = tester.element(find.byType(StartupAnimatedChild));
    final model = Provider.of<StartupModel>(context, listen: false);

    expect(model, isNotNull);
  });
}

const lastText = 'LAST TEXT';
Widget buildApp(StartupModel model) => buildMultiProviderWizardApp(
      providers: [ChangeNotifierProvider.value(value: model)],
      routes: {
        '/': WizardRoute(builder: (_) => const StartupAnimatedChild()),
        '/next': WizardRoute(builder: (_) => const Text(lastText)),
      },
    );
