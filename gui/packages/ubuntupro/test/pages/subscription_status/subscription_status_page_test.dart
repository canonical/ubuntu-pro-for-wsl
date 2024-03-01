import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_page.dart';
import 'package:wizard_router/wizard_router.dart';

void main() {
  group('subscription info', () {
    final client = FakeAgentApiClient();
    registerServiceInstance<AgentApiClient>(client);
    final info = SubscriptionInfo();
    testWidgets('user', (tester) async {
      info.ensureUser();
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.detachPro), findsOneWidget);
    });

    testWidgets('store', (tester) async {
      info.ensureMicrosoftStore();
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.manageSubscription), findsOneWidget);
    });

    testWidgets('organization', (tester) async {
      info.ensureOrganization();
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.orgManaged), findsOneWidget);
    });
  });
  testWidgets('creates a model', (tester) async {
    final info = ValueNotifier(SubscriptionInfo());
    info.value.ensureUser();

    final app = ChangeNotifierProvider.value(
      value: info,
      child: const MaterialApp(
        routes: {'/': SubscriptionStatusPage.create},
        localizationsDelegates: AppLocalizations.localizationsDelegates,
      ),
    );

    await tester.pumpWidget(app);
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(SubscriptionStatusPage));
    final model = Provider.of<SubscriptionStatusModel>(context, listen: false);

    expect(model, isNotNull);
  });
  group('sane navigation', () {
    Widget buildWizardApp(Map<String, WizardRoute> routes) {
      final info = ValueNotifier(SubscriptionInfo());
      info.value.ensureUser();
      return ChangeNotifierProvider.value(
        value: info,
        child: MaterialApp(
          builder: (context, _) => Wizard(routes: routes),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
        ),
      );
    }

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
        '/second': WizardRoute(
          builder: (_) => const Placeholder(),
        ),
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

Widget buildApp(SubscriptionInfo info, AgentApiClient client) {
  final model = SubscriptionStatusModel(info, client);
  return MaterialApp(
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    home: Provider.value(
      value: model,
      child: const SubscriptionStatusPage(),
    ),
  );
}

class FakeAgentApiClient extends Fake implements AgentApiClient {
  @override
  Future<void> applyLandscapeConfig(String config) async {}
  @override
  Future<SubscriptionInfo> applyProToken(String token) async {
    final info = SubscriptionInfo();
    info.ensureUser();
    return info;
  }
}
