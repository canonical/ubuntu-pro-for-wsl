import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_model.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_widgets.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import '../../utils/build_multiprovider_app.dart';
import '../../utils/token_samples.dart' as tks;
import '../../utils/url_launcher_mock.dart';
import 'subscribe_now_widgets_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  final launcher = FakeUrlLauncher();
  UrlLauncherPlatform.instance = launcher;

  testWidgets('launch web page', (tester) async {
    final theApp = buildApp(onApply: () {}, isExpanded: true);
    await tester.pumpWidget(theApp);

    expect(launcher.launched, isFalse);
    await tester.tapOnText(
      find.textRange.ofSubstring('ubuntu.com/pro/dashboard'),
    );
    await tester.pump();
    expect(launcher.launched, isTrue);
  });

  group('pro token input', () {
    group('basic flow', () {
      final theApp = buildApp(onApply: () {}, isExpanded: true);
      testWidgets('starts with no error', (tester) async {
        await tester.pumpWidget(theApp);

        final input = tester.firstWidget<TextField>(find.byType(TextField));

        expect(input.decoration!.errorText, isNull);
      });

      testWidgets('invalid non-empty tokens', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        for (final token in tks.invalidTokens) {
          await tester.enterText(inputField, token);
          await tester.pumpAndSettle();

          final errorText = find.descendant(
            of: inputField,
            matching: find.text(lang.tokenErrorInvalid),
          );
          expect(errorText, findsOne);
        }
      });

      testWidgets('empty token', (tester) async {
        // same as above test...
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        await tester.enterText(inputField, tks.invalidTokens[0]);
        await tester.pumpAndSettle();

        final errorText = find.descendant(
          of: inputField,
          matching: find.text(lang.tokenErrorInvalid),
        );
        expect(errorText, findsOne);

        // ...except when we delete the content we should have no more errors
        await tester.enterText(inputField, '');
        await tester.pumpAndSettle();
        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.error, isNull);
      });

      testWidgets('good token', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        await tester.enterText(inputField, tks.good);
        await tester.pumpAndSettle();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.error, isNull);
        final errorText = find.descendant(
          of: inputField,
          matching: find.text(lang.tokenErrorInvalid),
        );
        expect(errorText, findsNothing);
      });

      testWidgets('good token with spaces', (tester) async {
        await tester.pumpWidget(theApp);
        final inputField = find.byType(TextField);
        final context = tester.element(inputField);
        final lang = AppLocalizations.of(context);

        await tester.enterText(
          inputField,
          // good token plus a bunch of types of white spaces.
          ' ${tks.good} \u{00A0}\u{2000}\u{2002}\u{202F}\u{205F}\u{3000} ',
        );
        await tester.pumpAndSettle();

        final input = tester.firstWidget<TextField>(inputField);
        expect(input.decoration!.errorText, isNull);

        final errorText = find.descendant(
          of: inputField,
          matching: find.text(lang.tokenErrorInvalid),
        );
        expect(errorText, findsNothing);
      });
    });

    testWidgets('apply', (tester) async {
      var called = false;
      final theApp = buildApp(isExpanded: true, onApply: () => called = true);
      await tester.pumpWidget(theApp);

      final inputField = find.byType(TextField);

      await tester.enterText(inputField, tks.good);
      await tester.pumpAndSettle();

      expect(called, isFalse);
      // simulate an enter key/submission of the text field
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(called, isTrue);
    });

    testWidgets('apply on submit', (tester) async {
      var called = false;
      final theApp = buildApp(isExpanded: true, onApply: () => called = true);
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
    final theApp = buildApp(isExpanded: true, onApply: () {});
    await tester.pumpWidget(theApp);
    final context = tester.element(find.byType(ProTokenInputField));
    final lang = AppLocalizations.of(context);
    for (final value in TokenError.values) {
      // localize will throw if new values were added to the enum but not to the method.
      expect(() => value.localize(lang), returnsNormally);
    }
  });
}

Widget buildApp({required void Function() onApply, bool isExpanded = false}) {
  return buildSingleRouteMultiProviderApp(
    child: Scaffold(
      body: ProTokenInputField(onSubmit: onApply, isExpanded: isExpanded),
    ),
    providers: [
      ChangeNotifierProvider.value(
        value: SubscribeNowModel(MockAgentApiClient()),
      ),
    ],
  );
}
