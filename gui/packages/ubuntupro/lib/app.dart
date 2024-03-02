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
import 'pages/landscape/landscape_page.dart';
import 'pages/startup/agent_monitor.dart';
import 'pages/startup/startup_page.dart';
import 'pages/subscribe_now/subscribe_now_page.dart';
import 'pages/subscription_status/subscription_status_page.dart';
import 'routes.dart';

class Pro4WSLApp extends StatelessWidget {
  const Pro4WSLApp({super.key});

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) {
        return ChangeNotifierProvider(
          create: (_) => ValueNotifier(SubscriptionInfo()),
          child: MaterialApp(
            title: 'Ubuntu Pro',
            theme: customize(yaru.darkTheme),
            darkTheme: customize(yaru.darkTheme),
            debugShowCheckedModeBanner: false,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
            builder: (context, child) {
              return Wizard(
                routes: {
                  Routes.startup: WizardRoute(
                    builder: buildStartup,
                    onReplace: (_) async {
                      final subscriptionInfo =
                          context.read<ValueNotifier<SubscriptionInfo>>();
                      final client = getService<AgentApiClient>();
                      subscriptionInfo.value = await client.subscriptionInfo();

                      if (subscriptionInfo.value.whichSubscriptionType() !=
                          SubscriptionType.none) {
                        return Routes.subscriptionStatus;
                      }
                      return null;
                    },
                  ),
                  Routes.subscribeNow:
                      const WizardRoute(builder: SubscribeNowPage.create),
                  Routes.configureLandscape:
                      const WizardRoute(builder: LandscapePage.create),
                  Routes.subscriptionStatus: WizardRoute(
                    builder: SubscriptionStatusPage.create,
                    onReplace: (_) => Routes.subscribeNow,
                    onBack: (_) => Routes.subscribeNow,
                  ),
                  Routes.configureLandscapeLate: WizardRoute(
                    builder: (context) => LandscapePage.create(
                      context,
                      isLate: true,
                    ),
                  ),
                },
              );
            },
          ),
        );
      },
    );
  }
}

Widget buildStartup(BuildContext context) {
  return Provider<AgentStartupMonitor>(
    create: (context) => AgentStartupMonitor(
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

ThemeData? customize(ThemeData? theme) {
  if (theme == null) return null;
  const padding = MaterialStatePropertyAll<EdgeInsetsGeometry>(
    EdgeInsets.symmetric(vertical: 20.0, horizontal: 16.0),
  );
  const shape = MaterialStatePropertyAll<RoundedRectangleBorder>(
    RoundedRectangleBorder(
      borderRadius: BorderRadius.zero,
    ),
  );
  final textStyle = MaterialStatePropertyAll<TextStyle>(
    theme.textTheme.bodySmall!.copyWith(fontWeight: FontWeight.normal),
  );
  final filledButtonTheme = FilledButtonThemeData(
    style: theme.filledButtonTheme.style?.copyWith(
      shape: shape,
      padding: padding,
      textStyle: textStyle,
    ),
  );
  final elevatedButtonTheme = ElevatedButtonThemeData(
    style: theme.elevatedButtonTheme.style?.copyWith(
      shape: shape,
      padding: padding,
      textStyle: textStyle,
    ),
  );
  final outlinedButtonTheme = OutlinedButtonThemeData(
    style: theme.outlinedButtonTheme.style?.copyWith(
      shape: shape,
      padding: padding,
      textStyle: textStyle,
    ),
  );
  final buttonTheme = theme.buttonTheme.copyWith(
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.zero,
    ),
  );
  return theme.copyWith(
    buttonTheme: buttonTheme,
    filledButtonTheme: filledButtonTheme,
    elevatedButtonTheme: elevatedButtonTheme,
    outlinedButtonTheme: outlinedButtonTheme,
  );
}
