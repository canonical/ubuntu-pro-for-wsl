import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_model.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_page.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_widgets.dart';
import '../../utils/build_multiprovider_app.dart';
import 'subscribe_now_page_test.mocks.dart';
import 'token_samples.dart' as tks;

@GenerateMocks([SubscribeNowModel])
void main() {
  testWidgets('launch web page', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.purchaseAllowed()).thenReturn(true);
    var called = false;
    when(model.launchProWebPage()).thenAnswer((_) async {
      called = true;
    });
    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);

    expect(called, isFalse);
    final button = find.text(lang.about);
    await tester.tap(button);
    await tester.pump();
    expect(called, isTrue);
  });
  group('purchase button enabled by model', () {
    testWidgets('disabled', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed()).thenReturn(false);
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      // check that's the right button
      final button = find.ancestor(
        of: find.text(lang.subscribeNow),
        matching: find.byType(ElevatedButton),
      );
      expect(button, findsOneWidget);
      expect(tester.widget<ElevatedButton>(button).enabled, isFalse);
    });
    testWidgets('enabled', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed()).thenReturn(true);
      final app = buildApp(model, (_) {});
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(SubscribeNowPage));
      final lang = AppLocalizations.of(context);

      // check that's the right button
      final button = find.ancestor(
        of: find.text(lang.subscribeNow),
        matching: find.byType(ElevatedButton),
      );
      expect(button, findsOneWidget);
      expect(tester.widget<ElevatedButton>(button).enabled, isTrue);
    });
  });
  group('subscribe', () {
    testWidgets('calls back on success', (tester) async {
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed()).thenReturn(true);
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
      final button = find.text(lang.subscribeNow);
      await tester.tap(button);
      await tester.pump();
      expect(called, isTrue);
    });

    testWidgets('feedback on error', (tester) async {
      const purchaseError = PurchaseStatus.networkError;
      final model = MockSubscribeNowModel();
      when(model.purchaseAllowed()).thenReturn(true);
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
      final button = find.text(lang.subscribeNow);
      await tester.tap(button);
      await tester.pump();
      expect(find.byType(SnackBar), findsWidgets);
      expect(find.text(purchaseError.localize(lang)), findsWidgets);
      expect(called, isFalse);
    });
  });
  testWidgets('feedback when applying token', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.purchaseAllowed()).thenReturn(true);
    when(model.applyProToken(any)).thenAnswer((_) async {
      return SubscriptionInfo()..ensureUser();
    });
    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);

    // expands the collapsed input field group
    final toggle = find.byIcon(ProTokenInputField.expandIcon);
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    // enters a good token value
    final inputField = find.byType(TextField);
    await tester.enterText(inputField, tks.good);
    await tester.pump();

    // submits the input.
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);
    final button = find.text(lang.confirm);
    await tester.tap(button);
    await tester.pump();

    // asserts that feedback is shown
    expect(find.byType(SnackBar), findsOneWidget);
  });

  testWidgets('purchase status enum l10n', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.purchaseAllowed()).thenReturn(true);
    final app = buildApp(model, onSubscribeNoop);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);
    for (final value in PurchaseStatus.values) {
      // localize will throw if new values were added to the enum but not to the method.
      expect(() => value.localize(lang), returnsNormally);
    }
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
