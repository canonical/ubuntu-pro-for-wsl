import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:grpc/grpc.dart';
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
          home: Builder(
            builder: (context) {
              return const Wizard(
                routes: {
                  Routes.startup: WizardRoute(builder: buildStartup),
                  Routes.subscriptionStatus: WizardRoute(
                    builder: SubscriptionStatusPage.create,
                  ),
                },
              );
            },
          ),
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
      onClient: (client) async {
        registerServiceInstance<AgentApiClient>(client);
        final subscriptionInfo =
            context.read<ValueNotifier<SubscriptionInfo>>();
        // TODO: Remove this try-catch once the agent stop crashing due lack of MS Store access
        try {
          subscriptionInfo.value = await client.subscriptionInfo();
        } on GrpcError catch (err) {
          debugPrint('$err');
          debugPrintStack(maxFrames: 20);
        }
      },
    ),
    child: const StartupPage(),
  );
}

AgentApiClient defaultClient(int port) =>
    AgentApiClient(host: kDefaultHost, port: port);

Future<bool> launch() => launchAgent(kAgentRelativePath);
