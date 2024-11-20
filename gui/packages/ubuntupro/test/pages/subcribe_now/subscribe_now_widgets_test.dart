import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_widgets.dart';
import '../../utils/token_samples.dart' as tks;

void main() {
  group('pro token value', () {
    test('errors', () async {
      final value = ProTokenValue();

      value.update('');
      expect(value.errorOrNull, TokenError.empty);

      for (final token in tks.invalidTokens) {
        value.update(token);
        expect(value.errorOrNull, TokenError.invalid);
      }
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

        final button =
            tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));

        expect(button.enabled, isFalse);
      });

      testWidgets('invalid non-empty tokens', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        for (final token in tks.invalidTokens) {
          await tester.enterText(inputField, token);
          await tester.pump();

          final input = tester.firstWidget<TextField>(inputField);
          expect(input.decoration!.errorText, equals(lang.tokenErrorInvalid));

          final button =
              tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
          expect(button.enabled, isFalse);
        }
      });

      testWidgets('empty token', (tester) async {
        // same as above test...
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        await tester.enterText(inputField, tks.invalidTokens[0]);
        await tester.pump();

        var input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, equals(lang.tokenErrorInvalid));

        final button =
            tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
        expect(button.enabled, isFalse);

        // ...except when we delete the content we should have no more errors
        await tester.enterText(inputField, '');
        await tester.pump();
        input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNull);
        expect(button.enabled, isFalse);
      });

      testWidgets('good token', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);

        await tester.enterText(inputField, tks.good);
        await tester.pump();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNull);

        final button =
            tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
        expect(button.enabled, isTrue);
      });
      testWidgets('good token with spaces', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);

        await tester.enterText(
          inputField,
          // good token plus a bunch of types of white spaces.
          ' ${tks.good} \u{00A0}\u{2000}\u{2002}\u{202F}\u{205F}\u{3000} ',
        );
        await tester.pump();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNull);

        final button =
            tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
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
      final button = find.byType(ElevatedButton);
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

  testWidgets('token error enum l10n', (tester) async {
    final theApp = buildApp(isExpanded: true, onApply: (_) {});
    await tester.pumpWidget(theApp);
    final context = tester.element(find.byType(ProTokenInputField));
    final lang = AppLocalizations.of(context);
    for (final value in TokenError.values) {
      // localize will throw if new values were added to the enum but not to the method.
      expect(() => value.localize(lang), returnsNormally);
    }
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
