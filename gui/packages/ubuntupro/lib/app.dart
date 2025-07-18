import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

import 'core/agent_api_client.dart';
import 'core/agent_connection.dart';
import 'core/agent_monitor.dart';
import 'core/settings.dart';
import 'pages/landscape/landscape_page.dart';
import 'pages/landscape_skip/landscape_skip_page.dart';
import 'pages/startup/startup_page.dart';
import 'pages/subscribe_now/subscribe_now_page.dart';
import 'pages/subscription_status/subscription_status_page.dart';
import 'routes.dart';

class Pro4WSLApp extends StatelessWidget {
  const Pro4WSLApp(this.agentMonitor, this.settings, {super.key});
  final AgentStartupMonitor agentMonitor;
  final Settings settings;

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) {
        return MultiProvider(
          providers: [
            ChangeNotifierProvider(
              create: (_) => ValueNotifier(ConfigSources()),
            ),
            ChangeNotifierProvider(
              create: (_) => AgentConnection(agentMonitor),
            ),
          ],
          child: MaterialApp(
            title: 'Ubuntu Pro',
            theme: yaru.theme,
            darkTheme: yaru.darkTheme,
            debugShowCheckedModeBanner: false,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
            builder: (context, child) {
              return Wizard(
                routes: {
                  Routes.startup: WizardRoute(
                    builder:
                        (context) => Provider.value(
                          value: agentMonitor,
                          child: const StartupPage(),
                        ),
                    onReplace: (_) async {
                      final src = context.read<ValueNotifier<ConfigSources>>();
                      final client = getService<AgentApiClient>();
                      src.value = await client.configSources();

                      final subs = src.value.proSubscription;
                      if (!subs.hasNone()) {
                        return Routes.subscriptionStatus;
                      }
                      return null;
                    },
                  ),
                  Routes.subscribeNow: WizardRoute(
                    builder: SubscribeNowPage.create,
                    userData: settings.isStorePurchaseAllowed,
                    onNext: (_) {
                      final src = context.read<ValueNotifier<ConfigSources>>();
                      final landscape = src.value.landscapeSource;
                      if (landscape.hasOrganization()) {
                        // skip configuring Landscape.
                        return Routes.subscriptionStatus;
                      }
                      return null;
                    },
                  ),
                  if (settings.isLandscapeConfigurationEnabled) ...{
                    Routes.skipLandscape: WizardRoute(
                      builder: (_) => const LandscapeSkipPage(),
                      onNext: (settings) {
                        switch (settings.arguments as SkipEnum) {
                          case SkipEnum.skip:
                            return Routes.subscriptionStatus;
                          default:
                            return null;
                        }
                      },
                    ),
                    Routes.configureLandscape: const WizardRoute(
                      builder: LandscapePage.create,
                    ),
                    Routes.subscriptionStatus: WizardRoute(
                      builder: SubscriptionStatusPage.create,
                      onReplace: (_) => Routes.subscribeNow,
                      onBack: (_) => Routes.subscribeNow,
                      userData: true,
                    ),
                    Routes.configureLandscapeLate: WizardRoute(
                      builder:
                          (context) =>
                              LandscapePage.create(context, isLate: true),
                    ),
                  } else ...{
                    Routes.subscriptionStatus: WizardRoute(
                      builder: SubscriptionStatusPage.create,
                      onReplace: (_) => Routes.subscribeNow,
                      onBack: (_) => Routes.subscribeNow,
                      userData: false,
                    ),
                  },
                },
              );
            },
          ),
        );
      },
    );
  }
}
