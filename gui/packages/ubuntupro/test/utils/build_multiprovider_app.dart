import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nested/nested.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/core/agent_connection.dart';
import 'package:ubuntupro/core/agent_monitor.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

/// Simplifies creating app widgets which don't care about the behavior of status bar (majority of test cases).
Widget buildMultiProviderWizardApp({
  List<SingleChildWidget> providers = const [],
  required Map<String, WizardRoute> routes,
}) {
  return MultiProvider(
    providers: providers +
        [
          ChangeNotifierProvider<AgentConnection>(
            create: (_) => _MockAgentConnection(),
          ),
        ],
    child: YaruTheme(
      builder: (_, yaru, __) => MaterialApp(
        theme: yaru.theme,
        darkTheme: yaru.darkTheme,
        home: Wizard(routes: routes),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
      ),
    ),
  );
}

Widget buildSingleRouteMultiProviderApp({
  List<SingleChildWidget> providers = const [],
  required Widget child,
}) {
  return MultiProvider(
    providers: providers +
        [
          ChangeNotifierProvider<AgentConnection>(
            create: (_) => _MockAgentConnection(),
          ),
        ],
    child: YaruTheme(
      builder: (_, yaru, __) => MaterialApp(
        theme: yaru.theme,
        darkTheme: yaru.darkTheme,
        home: child,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
      ),
    ),
  );
}

// A dummy agent connection to satisfy the always-present status bar.
class _MockAgentStartupMonitor extends Fake implements AgentStartupMonitor {
  @override
  Stream<AgentState> start({
    Duration timeout = Duration.zero,
    Duration interval = Duration.zero,
  }) {
    return const Stream<AgentState>.empty();
  }

  @override
  bool addNewClientListener(AgentApiCallback cb) {
    return true;
  }
}

class _MockAgentConnection extends AgentConnection {
  _MockAgentConnection() : super(_MockAgentStartupMonitor());

  @override
  AgentConnectionState get state => AgentConnectionState.connected;
  @override
  Future<void> restartAgent() {
    return Future<void>.value();
  }
}
