import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:yaru/yaru.dart';

import 'constants.dart';
import 'core/agent_api_client.dart';
import 'launch_agent.dart';
import 'pages/startup/agent_monitor.dart';
import 'pages/startup/startup_page.dart';
import 'pages/subscription_status/subscription_status_page.dart';
import 'routes.dart';

class Pro4WindowsApp extends StatefulWidget {
  const Pro4WindowsApp({super.key});

  @override
  State<Pro4WindowsApp> createState() => _Pro4WindowsAppState();
}

class _Pro4WindowsAppState extends State<Pro4WindowsApp> {
  // A notifier that will allow child widgets to rebuild once new information
  // about the current subscription information arrives.
  final _subscriptionInfo = ValueNotifier(SubscriptionInfo());
  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) => ChangeNotifierProvider.value(
        value: _subscriptionInfo,
        child: MaterialApp(
          title: kAppName,
          theme: yaru.theme,
          darkTheme: yaru.darkTheme,
          debugShowCheckedModeBanner: false,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
          home: Provider<AgentStartupMonitor>(
            create: (context) => AgentStartupMonitor(
              appName: kAppName,
              addrFileName: kAddrFileName,
              agentLauncher: launch,
              clientFactory: defaultClient,
              onClient: _onClient,
            ),
            child: const StartupPage(nextRoute: Routes.subscriptionStatus),
          ),
          routes: const {
            Routes.subscriptionStatus: SubscriptionStatusPage.create,
          },
        ),
      ),
    );
  }

  Future<void> _onClient(AgentApiClient client) async {
    registerServiceInstance<AgentApiClient>(client);
    _subscriptionInfo.value = await client.subscriptionInfo();
  }
}

AgentApiClient defaultClient(int port) =>
    AgentApiClient(host: kDefaultHost, port: port);

Future<bool> launch() => launchAgent(kAgentRelativePath);
