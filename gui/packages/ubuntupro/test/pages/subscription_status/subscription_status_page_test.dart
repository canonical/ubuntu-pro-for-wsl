import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';
import 'package:ubuntupro/pages/subscription_status/susbcription_status_page.dart';

void main() {
  group('subscription info', () {
    final client = FakeAgentApiClient();
    final info = SubscriptionInfo();
    testWidgets('manual', (tester) async {
      info.ensureManual();
      info.userManaged = true;
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.detachPro), findsOneWidget);
    });

    testWidgets('store', (tester) async {
      info.ensureMicrosoftStore();
      info.userManaged = true;
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.manageSubscription), findsOneWidget);
    });

    testWidgets('organization', (tester) async {
      info.userManaged = false;
      final app = buildApp(info, client);

      await tester.pumpWidget(app);

      final context = tester.element(find.byType(SubscriptionStatusPage));
      final lang = AppLocalizations.of(context);

      expect(find.text(lang.orgManaged), findsOneWidget);
    });
  });
  testWidgets('creates a model', (tester) async {
    final mockClient = FakeAgentApiClient();
    final info = SubscriptionInfo();
    info.ensureManual();
    info.userManaged = true;
    registerServiceInstance<AgentApiClient>(mockClient);
    final app = Provider.value(
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

class FakeAgentApiClient extends Fake implements AgentApiClient {}
