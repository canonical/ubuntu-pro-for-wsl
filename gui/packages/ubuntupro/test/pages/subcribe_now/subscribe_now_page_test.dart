import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_model.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_page.dart';
import 'package:url_launcher_platform_interface/link.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import 'package:wizard_router/wizard_router.dart';

import '../../utils/build_multiprovider_app.dart';
import 'subscribe_now_page_test.mocks.dart';

@GenerateMocks([SubscribeNowModel])
void main() {
  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  // TODO: Sometimes the Column in the LandscapePage extends past the test environment's screen
  // due differences in font size between production and testing environments.
  // This should be resolved so that we don't have to specify a manual text scale factor.
  // See more: https://github.com/flutter/flutter/issues/108726#issuecomment-1205035859
  binding.platformDispatcher.textScaleFactorTestValue = 0.6;

  final launcher = FakeUrlLauncher();
  UrlLauncherPlatform.instance = launcher;

  testWidgets('launch web page', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.purchaseAllowed).thenReturn(true);

    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);

    expect(launcher.launched, isFalse);
    await tester.tapOnText(find.textRange.ofSubstring('Learn more'));
    await tester.pump();
    expect(launcher.launched, isTrue);
  });

  group('purchase button enabled by model', () {
    testWidgets('disabled', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed).thenReturn(false);
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      // check that's the right button
      final button = find.ancestor(
        of: find.text(lang.getUbuntuPro),
        matching: find.byType(ElevatedButton),
      );
      expect(button, findsNothing);
    });
    testWidgets('enabled', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed).thenReturn(true);
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      // check that's the right button
      final button = find.ancestor(
        of: find.text(lang.getUbuntuPro),
        matching: find.byType(ElevatedButton),
      );
      expect(button, findsOneWidget);
      expect(tester.widget<ElevatedButton>(button).enabled, isTrue);
    });
  });
  group('subscribe', () {
    testWidgets('calls back on success', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed).thenReturn(true);
      var called = false;
      when(model.purchaseSubscription()).thenAnswer((_) async {
        final info = SubscriptionInfo()..ensureMicrosoftStore();
        return info.right();
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
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed).thenReturn(true);
      var called = false;
      when(model.purchaseSubscription()).thenAnswer((_) async {
        return purchaseError.left();
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

  testWidgets('purchase status enum l10n', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.purchaseAllowed).thenReturn(true);
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
          create: (_) => ValueNotifier(
            ConfigSources(proSubscription: SubscriptionInfo()..ensureUser()),
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
    child: SubscribeNowPage(
      onSubscriptionUpdate: onSubs,
    ),
    providers: [
      Provider.value(value: model),
    ],
  );
}

void onSubscribeNoop(SubscriptionInfo _) {}

class FakeAgentApiClient extends Fake implements AgentApiClient {}

class FakeUrlLauncher extends UrlLauncherPlatform {
  bool launched = false;

  @override
  Future<bool> canLaunch(String url) async {
    return true;
  }

  @override
  Future<void> closeWebView() async {}

  @override
  Future<bool> launchUrl(String url, LaunchOptions options) async {
    launched = true;
    return true;
  }

  @override
  Future<bool> supportsCloseForMode(PreferredLaunchMode mode) async {
    return true;
  }

  @override
  Future<bool> supportsMode(PreferredLaunchMode mode) async {
    return true;
  }

  @override
  Future<bool> launch(
    String url, {
    required bool useSafariVC,
    required bool useWebView,
    required bool enableJavaScript,
    required bool enableDomStorage,
    required bool universalLinksOnly,
    required Map<String, String> headers,
    String? webOnlyWindowName,
  }) async {
    launched = true;
    return true;
  }

  @override
  LinkDelegate? get linkDelegate => null;
}
