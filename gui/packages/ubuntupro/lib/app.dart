import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

import 'constants.dart';
import 'core/agent_api_client.dart';
import 'launch_agent.dart';
import 'pages/startup/agent_monitor.dart';
import 'pages/startup/startup_page.dart';
import 'pages/subscription_status/subscription_status_page.dart';
import 'routes.dart';

class Pro4WindowsApp extends StatelessWidget {
  const Pro4WindowsApp({super.key});

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) => ChangeNotifierProvider(
        create: (_) => ValueNotifier(SubscriptionInfo()),
        child: MaterialApp(
          title: kAppName,
          theme: yaru.theme,
          darkTheme: yaru.darkTheme,
          debugShowCheckedModeBanner: false,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
          builder: (context, child) {
            return Wizard(
              routes: {
                Routes.startup: const WizardRoute(builder: buildStartup),
                Routes.subscriptionStatus: WizardRoute(
                  builder: SubscriptionStatusPage.create,
                  onLoad: (_) async {
                    final client = getService<AgentApiClient>();
                    final subscriptionInfo =
                        context.read<ValueNotifier<SubscriptionInfo>>();

                    subscriptionInfo.value = await client.subscriptionInfo();

                    // never skip this page.
                    return true;
                  },
                ),
              },
            );
          },
        ),
      ),
    );
  }
}

Widget buildStartup(BuildContext context) {
  return Provider<AgentStartupMonitor>(
    create: (context) => AgentStartupMonitor(
      appName: kAppName,
      addrFileName: kAddrFileName,
      agentLauncher: launch,
      clientFactory: defaultClient,
      onClient: registerServiceInstance<AgentApiClient>,
    ),
    child: const StartupPage(),
  );
}

AgentApiClient defaultClient(int port) =>
    AgentApiClient(host: kDefaultHost, port: port);

Future<bool> launch() => launchAgent(kAgentRelativePath);
