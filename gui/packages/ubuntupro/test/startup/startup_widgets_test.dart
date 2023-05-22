import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/pages/startup/startup_widgets.dart';

void main() {
  const message = 'Hello';
  MaterialApp buildApp(Widget home) => MaterialApp(
        home: home,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
      );
  testWidgets('inprogress no appbar', (tester) async {
    await tester.pumpWidget(buildApp(const StartupInProgressWidget(message)));
    expect(find.byType(AppBar), findsNothing);
  });
  testWidgets('retry shows appbar', (tester) async {
    await tester.pumpWidget(
      buildApp(
        const StartupRetryWidget(
          message: message,
          retry: Icon(Icons.check),
        ),
      ),
    );
    expect(find.byType(AppBar), findsOneWidget);
  });

  testWidgets('error also shows appbar', (tester) async {
    await tester.pumpWidget(buildApp(const StartupErrorWidget(message)));
    expect(find.byType(AppBar), findsOneWidget);
  });
}
