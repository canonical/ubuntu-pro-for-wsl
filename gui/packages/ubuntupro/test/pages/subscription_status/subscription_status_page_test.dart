import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_page.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import 'package:wizard_router/wizard_router.dart';

import '../../utils/build_multiprovider_app.dart';
import '../../utils/url_launcher_mock.dart';

void main() {
  group('subscription info:', () {
    final client = FakeAgentApiClient();
    registerServiceInstance<AgentApiClient>(client);
    final info = SubscriptionInfo();

    group('footer links:', () {
      final landscape = LandscapeSource()..ensureOrganization();
      testWidgets('user', (tester) async {
        final launcher = FakeUrlLauncher();
        UrlLauncherPlatform.instance = launcher;

        info.ensureUser();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(launcher.launched, isFalse);
        await tester.tapOnText(
          find.textRange.ofSubstring(lang.manageUbuntuPro),
        );
        await tester.pump();
        expect(launcher.launched, isTrue);
      });

      testWidgets('store', (tester) async {
        final launcher = FakeUrlLauncher();
        UrlLauncherPlatform.instance = launcher;

        info.ensureMicrosoftStore();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(launcher.launched, isFalse);
        await tester.tapOnText(
          find.textRange.ofSubstring(lang.manageUbuntuPro),
        );
        await tester.pump();
        expect(launcher.launched, isTrue);
      });
    });

    group('org landscape:', () {
      final landscape = LandscapeSource()..ensureOrganization();
      testWidgets('user', (tester) async {
        info.ensureUser();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.detachPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });

      testWidgets('store', (tester) async {
        info.ensureMicrosoftStore();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });

      testWidgets('organization', (tester) async {
        info.ensureOrganization();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsNothing);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });
    });

    group('landscape:', () {
      testWidgets('user', (tester) async {
        final landscape = LandscapeSource()..ensureNone();
        info.ensureUser();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.detachPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsOneWidget);
      });

      testWidgets('store', (tester) async {
        final landscape = LandscapeSource()..ensureUser();
        info.ensureMicrosoftStore();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsOneWidget);
      });

      testWidgets('organization', (tester) async {
        final landscape = LandscapeSource();
        info.ensureOrganization();
        final app = buildApp(info, landscape, client);

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsNothing);
        expect(find.text(lang.landscapeConfigureButton), findsOneWidget);
      });
    });

    group('landscape feature disabled:', () {
      testWidgets('user', (tester) async {
        final landscape = LandscapeSource()..ensureNone();
        info.ensureUser();
        final app = buildApp(
          info,
          landscape,
          client,
          landscapeFeatureIsEnabled: false,
        );

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.detachPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });

      testWidgets('store', (tester) async {
        final landscape = LandscapeSource()..ensureUser();
        info.ensureMicrosoftStore();
        final app = buildApp(
          info,
          landscape,
          client,
          landscapeFeatureIsEnabled: false,
        );

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsOneWidget);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });

      testWidgets('organization', (tester) async {
        final landscape = LandscapeSource();
        info.ensureOrganization();
        final app = buildApp(
          info,
          landscape,
          client,
          landscapeFeatureIsEnabled: false,
        );

        await tester.pumpWidget(app);

        final context = tester.element(find.byType(SubscriptionStatusPage));
        final lang = AppLocalizations.of(context);

        expect(find.text(lang.manageUbuntuPro), findsNothing);
        expect(find.text(lang.landscapeConfigureButton), findsNothing);
      });
    });
  });
  testWidgets('creates a model', (tester) async {
    final app = buildMultiProviderWizardApp(
      routes: {'/': const WizardRoute(builder: SubscriptionStatusPage.create)},
      providers: [
        ChangeNotifierProvider(
          create: (_) => ValueNotifier(
            ConfigSources(
              proSubscription: SubscriptionInfo()..ensureUser(),
            ),
          ),
        ),
      ],
    );

    await tester.pumpWidget(app);
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(SubscriptionStatusPage));
    final model = Provider.of<SubscriptionStatusModel>(context, listen: false);

    expect(model, isNotNull);
  });
  group('sane navigation', () {
    testWidgets('no backwards', (tester) async {
      var replaced = false;
      var retrocessed = false;

      final app = buildWizardApp({
        '/': WizardRoute(
          builder: SubscriptionStatusPage.create,
          onReplace: (_) async {
            replaced = true;
            return null;
          },
          onBack: (_) async {
            retrocessed = true;
            return null;
          },
        ),
        '/second': WizardRoute(builder: (_) => const Placeholder()),
      });

      await tester.pumpWidget(app);
      await tester.pumpAndSettle();

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);
      final detach = find.text(lang.detachPro);

      expect(detach, findsOneWidget);
      await tester.tap(detach);
      await tester.pumpAndSettle();

      expect(replaced, isTrue);
      expect(retrocessed, isFalse);
    });

    testWidgets('backwards', (tester) async {
      var replaced = false;
      var retrocessed = false;
      const clickMe = 'Click me';

      final app = buildWizardApp({
        '/': WizardRoute(
          builder: (context) => Center(
            child: FilledButton(
              onPressed: () {
                Wizard.of(context).next();
              },
              child: const Text(clickMe),
            ),
          ),
        ),
        '/second': WizardRoute(
          builder: SubscriptionStatusPage.create,
          onReplace: (_) async {
            replaced = true;
            return null;
          },
          onBack: (_) async {
            retrocessed = true;
            return null;
          },
        ),
      });

      await tester.pumpWidget(app);
      await tester.pumpAndSettle();

      final clickButton = find.text(clickMe);
      await tester.tap(clickButton);
      await tester.pumpAndSettle();

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);
      final detach = find.text(lang.detachPro);

      expect(detach, findsOneWidget);
      await tester.tap(detach);
      await tester.pumpAndSettle();

      expect(retrocessed, isTrue);
      expect(replaced, isFalse);
    });
  });
}

Widget buildApp(
  SubscriptionInfo info,
  LandscapeSource landscape,
  AgentApiClient client, {
  bool landscapeFeatureIsEnabled = true,
}) {
  return buildMultiProviderWizardApp(
    routes: {'/': WizardRoute(builder: (_) => const SubscriptionStatusPage())},
    providers: [
      Provider(
        create: (_) => SubscriptionStatusModel(
          ConfigSources(proSubscription: info, landscapeSource: landscape),
          client,
          canConfigureLandscape: landscapeFeatureIsEnabled,
        ),
      ),
    ],
  );
}

Widget buildWizardApp(Map<String, WizardRoute> routes) {
  return buildMultiProviderWizardApp(
    routes: routes,
    providers: [
      ChangeNotifierProvider(
        create: (_) => ValueNotifier(
          ConfigSources(proSubscription: SubscriptionInfo()..ensureUser()),
        ),
      ),
    ],
  );
}

class FakeAgentApiClient extends Fake implements AgentApiClient {
  @override
  Future<LandscapeSource> applyLandscapeConfig(String config) async {
    return LandscapeSource()..ensureUser();
  }

  @override
  Future<SubscriptionInfo> applyProToken(String token) async {
    final info = SubscriptionInfo();
    info.ensureUser();
    return info;
  }
}
