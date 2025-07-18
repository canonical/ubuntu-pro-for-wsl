import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_model.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_page.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru_test/yaru_test.dart';

import '../../utils/build_multiprovider_app.dart';
import '../../utils/token_samples.dart';
import '../../utils/url_launcher_mock.dart';
import 'subscribe_now_page_test.mocks.dart';

@GenerateMocks([AgentApiClient, P4wMsStore])
void main() {
  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  // TODO: Sometimes the Column in the LandscapePage extends past the test environment's screen
  // due differences in font size between production and testing environments.
  // This should be resolved so that we don't have to specify a manual text scale factor.
  // See more: https://github.com/flutter/flutter/issues/108726#issuecomment-1205035859
  binding.platformDispatcher.textScaleFactorTestValue = 0.6;

  testWidgets('launch web page', (tester) async {
    final launcher = FakeUrlLauncher();
    UrlLauncherPlatform.instance = launcher;
    final model = SubscribeNowModel(
      MockAgentApiClient(),
      isPurchaseAllowed: true,
    );

    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);

    expect(launcher.launched, isFalse);
    await tester.tapOnText(find.textRange.ofSubstring(lang.learnMore));
    await tester.pump();
    expect(launcher.launched, isTrue);
  });

  testWidgets('launch subscribe page', (tester) async {
    final launcher = FakeUrlLauncher();
    UrlLauncherPlatform.instance = launcher;
    final model = SubscribeNowModel(
      MockAgentApiClient(),
      isPurchaseAllowed: false,
    );

    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);

    expect(launcher.launched, isFalse);
    await tester.tap(find.button(lang.getUbuntuPro));
    await tester.pumpAndSettle();
    expect(launcher.launched, isTrue);
  });

  group('subscribe', () {
    testWidgets('calls back on success', (tester) async {
      final store = MockP4wMsStore();
      final client = MockAgentApiClient();
      final model = SubscribeNowModel(
        client,
        isPurchaseAllowed: true,
        store: store,
      );
      var called = false;
      when(client.notifyPurchase()).thenAnswer((_) async {
        return SubscriptionInfo()..ensureMicrosoftStore().left();
      });
      when(store.purchaseSubscription(any)).thenAnswer((_) async {
        return PurchaseStatus.succeeded;
      });
      final app = buildApp(model, (_) {
        called = true;
      });
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      expect(called, isFalse);
      final button = find.text(lang.getUbuntuPro);
      await tester.tap(button);
      await tester.pump();
      expect(called, isTrue);
    });

    testWidgets('feedback on error', (tester) async {
      const purchaseError = PurchaseStatus.networkError;
      final store = MockP4wMsStore();
      final client = MockAgentApiClient();
      final model = SubscribeNowModel(
        client,
        isPurchaseAllowed: true,
        store: store,
      );
      var called = false;
      when(store.purchaseSubscription(any)).thenAnswer((_) async {
        return purchaseError;
      });
      final app = buildApp(model, (_) {
        called = true;
      });
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      expect(called, isFalse);
      final button = find.text(lang.getUbuntuPro);
      await tester.tap(button);
      await tester.pump();
      expect(find.byType(SnackBar), findsWidgets);
      expect(find.text(purchaseError.localize(lang)), findsWidgets);
      expect(called, isFalse);
    });
  });

  group('attach', () {
    testWidgets('submit on attach', (tester) async {
      var applied = false;
      final store = MockP4wMsStore();
      final client = MockAgentApiClient();
      final model = SubscribeNowModel(
        client,
        isPurchaseAllowed: true,
        store: store,
      );
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      when(client.applyProToken(good)).thenAnswer((_) async {
        applied = true;
        return SubscriptionInfo();
      });

      final attach = find.button(lang.attach);
      expect(tester.firstWidget<ButtonStyleButton>(attach).enabled, isFalse);

      final input = find.textField(lang.tokenInputHint);
      await tester.enterText(input, good);
      await tester.pumpAndSettle();

      expect(tester.firstWidget<ButtonStyleButton>(attach).enabled, isTrue);

      expect(applied, isFalse);
      await tester.tap(attach);
      await tester.pump();
      expect(applied, isTrue);
    });

    testWidgets('no submit with error', (tester) async {
      var applied = false;
      final store = MockP4wMsStore();
      final client = MockAgentApiClient();
      final model = SubscribeNowModel(
        client,
        isPurchaseAllowed: true,
        store: store,
      );
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      when(client.applyProToken(invalidTokens[0])).thenAnswer((_) async {
        applied = true;
        return SubscriptionInfo();
      });

      final attach = find.button(lang.attach);
      expect(tester.firstWidget<ButtonStyleButton>(attach).enabled, isFalse);

      final input = find.textField(lang.tokenInputHint);
      await tester.enterText(input, invalidTokens[0]);
      await tester.pumpAndSettle();

      expect(tester.firstWidget<ButtonStyleButton>(attach).enabled, isFalse);

      expect(applied, isFalse);
      await tester.tap(attach);
      await tester.pump();
      expect(applied, isFalse);
    });
  });

  testWidgets('purchase status enum l10n', (tester) async {
    final model = SubscribeNowModel(
      MockAgentApiClient(),
      isPurchaseAllowed: true,
    );
    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);
    for (final value in PurchaseStatus.values) {
      // localize will throw if new values were added to the enum but not to the method.
      expect(() => value.localize(lang), returnsNormally);
    }
  });

  testWidgets('creates a model', (tester) async {
    registerServiceInstance<AgentApiClient>(FakeAgentApiClient());
    final app = buildMultiProviderWizardApp(
      routes: {'/': const WizardRoute(builder: SubscribeNowPage.create)},
      providers: [
        ChangeNotifierProvider(
          create:
              (_) => ValueNotifier(
                ConfigSources(
                  proSubscription: SubscriptionInfo()..ensureUser(),
                ),
              ),
        ),
      ],
    );

    await tester.pumpWidget(app);
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(SubscribeNowPage));
    final model = Provider.of<SubscribeNowModel>(context, listen: false);

    expect(model, isNotNull);
  });
}

Widget buildApp(
  SubscribeNowModel model,
  void Function(SubscriptionInfo) onSubs,
) {
  return buildSingleRouteMultiProviderApp(
    child: SubscribeNowPage(onSubscriptionUpdate: onSubs),
    providers: [ChangeNotifierProvider.value(value: model)],
  );
}

void onSubscribeNoop(SubscriptionInfo _) {}

class FakeAgentApiClient extends Fake implements AgentApiClient {}
