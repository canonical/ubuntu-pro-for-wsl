import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/pages/subscription_status/subscribe_now_page.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_model.dart';
import 'package:yaru/yaru.dart';
import 'subscribe_now_page_test.mocks.dart';
import 'token_samples.dart' as tks;

@GenerateMocks([SubscribeNowModel])
void main() {
  testWidgets('launch web page', (tester) async {
    final model = MockSubscribeNowModel();
    var called = false;
    when(model.launchProWebPage()).thenAnswer((_) async {
      called = true;
    });
    final app = buildApp(model);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);

    expect(called, isFalse);
    final button = find.text(lang.learnMore);
    await tester.tap(button);
    await tester.pump();
    expect(called, isTrue);
  });
  testWidgets('subscribe', (tester) async {
    final model = MockSubscribeNowModel();
    var called = false;
    when(model.purchaseSubscription()).thenAnswer((_) async {
      called = true;
    });
    final app = buildApp(model);
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);

    expect(called, isFalse);
    final button = find.text(lang.subscribeNow);
    await tester.tap(button);
    await tester.pump();
    expect(called, isTrue);
  });
  testWidgets('feedback when applying token', (tester) async {
    final model = MockSubscribeNowModel();
    when(model.applyProToken(any)).thenAnswer((_) async {
      return;
    });
    final app = buildApp(model);
    await tester.pumpWidget(app);

    // expands the collapsed input field group
    final toggle = find.byType(IconButton);
    await tester.tap(toggle);
    await tester.pumpAndSettle();

    // enters a good token value
    final inputField = find.byType(TextField);
    await tester.enterText(inputField, tks.good);
    await tester.pump();

    // submits the input.
    final context = tester.element(find.byType(SubscribeNowPage));
    final lang = AppLocalizations.of(context);
    final button = find.text(lang.apply);
    await tester.tap(button);
    await tester.pump();

    // asserts that feedback is shown
    expect(find.byType(SnackBar), findsOneWidget);
  });
}

Widget buildApp(SubscriptionStatusModel model) {
  return YaruTheme(
    builder: (context, yaru, child) => MaterialApp(
      theme: yaru.theme,
      darkTheme: yaru.darkTheme,
      home: Scaffold(
        body: Provider.value(value: model, child: const SubscribeNowPage()),
      ),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    ),
  );
}
