import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';
import 'package:ubuntupro/pages/landscape/landscape_page.dart';
import 'package:ubuntupro/pages/widgets/page_widgets.dart';
import 'package:yaru/yaru.dart';
import 'package:yaru_test/yaru_test.dart';

import 'landscape_page_test.mocks.dart';

@GenerateMocks([LandscapeModel])
void main() {
  testWidgets('launch Landscape page', (tester) async {
    final model = MockLandscapeModel();

    var launched = false;
    when(model.launchLandscapeWebPage()).thenAnswer((_) async {
      launched = true;
    });
    when(model.fqdnError).thenReturn(false);
    when(model.fileError).thenReturn(FileError.none);
    when(await model.applyConfig()).thenReturn(false);
    when(model.selected).thenReturn(LandscapeConfigType.manual);
    when(model.receivedInput).thenReturn(false);

    final app = buildApp(model);
    await tester.pumpWidget(app);

    expect(launched, isFalse);
    final button = tester.widget<MarkdownBody>(find.byType(MarkdownBody));
    button.onTapLink!('', '', '');
    await tester.pump();
    expect(launched, isTrue);
  });

  group('input sections', () {
    testWidgets('default state', (tester) async {
      final model = MockLandscapeModel();

      when(model.fqdnError).thenReturn(false);
      when(model.fileError).thenReturn(FileError.none);
      when(await model.applyConfig()).thenReturn(false);
      when(model.selected).thenReturn(LandscapeConfigType.manual);
      when(model.receivedInput).thenReturn(false);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<FilledButton>(continueButton).enabled, isFalse);

      final radio1 = find.byWidgetPredicate(
        (widget) =>
            widget is Radio && widget.value == LandscapeConfigType.manual,
      );
      expect(radio1, findsOne);
      expect(
        tester.widget<Radio>(radio1).groupValue == LandscapeConfigType.manual,
        isTrue,
      );

      final radio2 = find.byWidgetPredicate(
        (widget) => widget is Radio && widget.value == LandscapeConfigType.file,
      );
      expect(radio2, findsOne);
      expect(
        tester.widget<Radio>(radio2).groupValue == LandscapeConfigType.manual,
        isTrue,
      );
    });

    testWidgets('continue enabled', (tester) async {
      final model = MockLandscapeModel();

      when(model.fqdnError).thenReturn(false);
      when(model.fileError).thenReturn(FileError.none);
      when(model.receivedInput).thenReturn(true);
      when(await model.applyConfig()).thenReturn(true);
      when(model.selected).thenReturn(LandscapeConfigType.manual);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<FilledButton>(continueButton).enabled, isTrue);
    });
  });

  group('input', () {
    testWidgets('calls back on success', (tester) async {
      final model = MockLandscapeModel();

      var applied = false;
      when(model.fqdnError).thenReturn(false);
      when(model.fileError).thenReturn(FileError.none);
      when(model.receivedInput).thenReturn(true);
      when(model.applyConfig()).thenAnswer((_) async {
        applied = true;
        return true;
      });
      when(model.selected).thenReturn(LandscapeConfigType.manual);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(applied, isFalse);
      await tester.tap(continueButton);
      await tester.pump();
      expect(applied, isTrue);
    });

    testWidgets('feedback on manual error', (tester) async {
      final model = MockLandscapeModel();
      await tester.binding.setSurfaceSize(const Size(900, 600));

      when(model.fqdnError).thenReturn(true);
      when(model.fileError).thenReturn(FileError.none);
      when(model.receivedInput).thenReturn(true);
      when(model.selected).thenReturn(LandscapeConfigType.manual);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFQDNLabel),
        matching: find.byType(TextField),
      );
      expect(fqdnInput, findsOne);
      final errorText = find.text(lang.landscapeFQDNError);
      expect(errorText, findsOne);
    });

    testWidgets('feedback on file error', (tester) async {
      final model = MockLandscapeModel();
      await tester.binding.setSurfaceSize(const Size(900, 600));

      when(model.fqdnError).thenReturn(false);
      when(model.fileError).thenReturn(FileError.notFound);
      when(model.receivedInput).thenReturn(true);
      when(model.selected).thenReturn(LandscapeConfigType.file);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFileLabel),
        matching: find.byType(TextField),
      );
      expect(fqdnInput, findsOne);
      final errorText = find.text(lang.landscapeFileNotFound);
      expect(errorText, findsOne);
    });
  });
}

Widget buildApp(
  LandscapeModel model,
) {
  return YaruTheme(
    builder: (context, yaru, child) => MaterialApp(
      theme: yaru.theme,
      darkTheme: yaru.darkTheme,
      home: Scaffold(
        body: ChangeNotifierProvider<LandscapeModel>(
          create: (_) => model,
          child: LandscapePage(
            onApplyConfig: () {},
          ),
        ),
      ),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    ),
  );
}
