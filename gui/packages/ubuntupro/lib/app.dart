import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:yaru/yaru.dart';

import 'constants.dart';
import 'core/agent_api_client.dart';
import 'launch_agent.dart';
import 'pages/enter_token/enter_token_page.dart';
import 'pages/startup/startup_page.dart';
import 'routes.dart';

class Pro4WindowsApp extends StatelessWidget {
  const Pro4WindowsApp({super.key});

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) => MaterialApp(
        title: kAppName,
        theme: yaru.theme,
        darkTheme: yaru.darkTheme,
        debugShowCheckedModeBanner: false,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
        home: const StartupPage(
          launcher: launch,
          nextRoute: Routes.enterToken,
          clientFactory: defaultClient,
          onClient: registerServiceInstance<AgentApiClient>,
        ),
        routes: const {
          Routes.enterToken: EnterProTokenPage.create,
        },
      ),
    );
  }
}

AgentApiClient defaultClient(int port) =>
    AgentApiClient(host: kDefaultHost, port: port);

Future<bool> launch() => launchAgent(kAgentRelativePath);
