import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_widgets.dart';
import 'token_samples.dart' as tks;

void main() {
  group('pro token value', () {
    test('errors', () async {
      final value = ProTokenValue();

      value.update('');

      expect(value.errorOrNull, TokenError.empty);

      value.update(tks.tooShort);

      expect(value.errorOrNull, TokenError.tooShort);

      value.update(tks.tooLong);

      expect(value.errorOrNull, TokenError.tooLong);

      value.update(tks.invalidPrefix);

      expect(value.errorOrNull, TokenError.invalidPrefix);

      value.update(tks.invalidEncoding);

      expect(value.errorOrNull, TokenError.invalidEncoding);
    });
    test('accessors on success', () {
      final value = ProTokenValue();
      final tokenInstance = ProToken.create(tks.good).orNull();

      value.update(tks.good);

      expect(value.hasError, isFalse);
      expect(value.errorOrNull, isNull);
      expect(value.token, tks.good);
      expect(value.valueOrNull!.value, tks.good);
      expect(value.valueOrNull, tokenInstance);
      expect(value.value, equals(ProToken.create(tks.good)));
    });

    test('notify listeners', () {
      final value = ProTokenValue();
      var notified = false;
      value.addListener(() {
        notified = true;
      });

      value.update(tks.good);

      expect(notified, isTrue);
    });
  });

  group('pro token input', () {
    testWidgets('collapsed by default', (tester) async {
      final app = MaterialApp(
        home: Scaffold(
          body: ProTokenInputField(
            onApply: (_) {},
          ),
        ),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
      );

      await tester.pumpWidget(app);
      expect(find.byType(TextField).hitTestable(), findsNothing);

      final toggle = find.byType(IconButton);
      await tester.tap(toggle);
      await tester.pumpAndSettle();
      expect(find.byType(TextField).hitTestable(), findsOneWidget);
    });

    group('basic flow', () {
      final theApp = buildApp(onApply: (_) {}, isExpanded: true);
      testWidgets('starts with no error', (tester) async {
        await tester.pumpWidget(theApp);

        final input = tester.firstWidget<TextField>(find.byType(TextField));

        expect(input.decoration!.errorText, isNull);
      });
      testWidgets('starts with button disabled', (tester) async {
        await tester.pumpWidget(theApp);

        final button = tester.firstWidget<TextButton>(find.byType(TextButton));

        expect(button.enabled, isFalse);
      });

      testWidgets('too short token', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);

        await tester.enterText(inputField, tks.tooShort);
        await tester.pump();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNotNull);
        expect(input.decoration!.errorText, contains('too short'));

        final button = tester.firstWidget<TextButton>(find.byType(TextButton));
        expect(button.enabled, isFalse);
      });

      testWidgets('too long token', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);

        await tester.enterText(inputField, tks.tooLong);
        await tester.pump();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNotNull);
        expect(input.decoration!.errorText, contains('too long'));

        final button = tester.firstWidget<TextButton>(find.byType(TextButton));
        expect(button.enabled, isFalse);
      });

      testWidgets('good token', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);

        await tester.enterText(inputField, tks.good);
        await tester.pump();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNull);

        final button = tester.firstWidget<TextButton>(find.byType(TextButton));
        expect(button.enabled, isTrue);
      });
    });
    testWidgets('apply', (tester) async {
      var called = false;
      final theApp = buildApp(isExpanded: true, onApply: (_) => called = true);
      await tester.pumpWidget(theApp);

      final inputField = find.byType(TextField);

      await tester.enterText(inputField, tks.good);
      await tester.pump();

      expect(called, isFalse);
      final button = find.byType(TextButton);
      await tester.tap(button);
      await tester.pumpAndSettle();
      expect(called, isTrue);
    });

    testWidgets('apply on submit', (tester) async {
      var called = false;
      final theApp = buildApp(isExpanded: true, onApply: (_) => called = true);
      await tester.pumpWidget(theApp);

      final textFieldFinder = find.byType(TextField);

      expect(called, isFalse);
      await tester.enterText(textFieldFinder, tks.good);
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(called, isTrue);
    });
  });
}

MaterialApp buildApp({
  required void Function(ProToken) onApply,
  bool isExpanded = false,
}) {
  return MaterialApp(
    home: Scaffold(
      body: ProTokenInputField(
        onApply: onApply,
        isExpanded: isExpanded,
      ),
    ),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
  );
}
