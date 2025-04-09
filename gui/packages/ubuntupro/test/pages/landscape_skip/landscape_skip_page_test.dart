import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/pages/landscape_skip/landscape_skip_page.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';
import 'package:yaru_test/yaru_test.dart';

import '../../utils/build_multiprovider_app.dart';
import '../../utils/url_launcher_mock.dart';

void main() {
  final launcher = FakeUrlLauncher();
  UrlLauncherPlatform.instance = launcher;

  testWidgets('default state', (tester) async {
    final app = buildApp();
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(LandscapeSkipPage));
    final lang = AppLocalizations.of(context);

    final backButton = find.button(lang.buttonBack);
    expect(backButton, findsOne);
    // for the purposes of these tests, we don't really care what kind of button
    // it is, just that it's enabled
    expect(tester.widget<ButtonStyleButton>(backButton).enabled, isTrue);

    final nextButton = find.button(lang.buttonNext);
    expect(nextButton, findsOne);
    expect(tester.widget<ButtonStyleButton>(nextButton).enabled, isTrue);

    final skipRadioTile = find.ancestor(
      of: find.text(lang.landscapeSkip),
      matching: find.byType(YaruSelectableContainer),
    );
    expect(
      tester.widget<YaruSelectableContainer>(skipRadioTile).selected,
      isTrue,
    );

    final registerRadioTile = find.ancestor(
      of: find.text(lang.landscapeSkipRegister),
      matching: find.byType(YaruSelectableContainer),
    );
    expect(
      tester.widget<YaruSelectableContainer>(registerRadioTile).selected,
      isFalse,
    );
  });

  testWidgets('tiles selectable', (tester) async {
    final app = buildApp();
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(LandscapeSkipPage));
    final lang = AppLocalizations.of(context);

    final skipRadioTile = find.ancestor(
      of: find.text(lang.landscapeSkip),
      matching: find.byType(YaruSelectableContainer),
    );
    final registerRadioTile = find.ancestor(
      of: find.text(lang.landscapeSkipRegister),
      matching: find.byType(YaruSelectableContainer),
    );

    await tester.tap(registerRadioTile);
    await tester.pump();
    expect(
      tester.widget<YaruSelectableContainer>(registerRadioTile).selected,
      isTrue,
    );
    expect(
      tester.widget<YaruSelectableContainer>(skipRadioTile).selected,
      isFalse,
    );

    await tester.tap(skipRadioTile);
    await tester.pump();
    expect(
      tester.widget<YaruSelectableContainer>(registerRadioTile).selected,
      isFalse,
    );
    expect(
      tester.widget<YaruSelectableContainer>(skipRadioTile).selected,
      isTrue,
    );
  });

  testWidgets('launch web page', (tester) async {
    final app = buildApp();
    await tester.pumpWidget(app);
    final context = tester.element(find.byType(LandscapeSkipPage));
    final lang = AppLocalizations.of(context);

    expect(launcher.launched, isFalse);
    await tester.tapOnText(find.textRange.ofSubstring(lang.learnMore));
    await tester.pump();
    expect(launcher.launched, isTrue);
  });
}

Widget buildApp() {
  return buildMultiProviderWizardApp(
    routes: {'/': WizardRoute(builder: (_) => const LandscapeSkipPage())},
  );
}
