import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_widgets.dart';

void main() {
  group('subscription status', () {
    const caption = 'my caption';
    const buttonName = 'my button';

    testWidgets('caption', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: SubscriptionStatus(caption: caption),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
        ),
      );

      expect(find.text(caption), findsOneWidget);
    });

    testWidgets('action button', (tester) async {
      var clicked = false;
      await tester.pumpWidget(
        MaterialApp(
          home: SubscriptionStatus(
            caption: caption,
            actionButtons: [
              TextButton(
                onPressed: () => clicked = true,
                child: const Text(buttonName),
              ),
            ],
          ),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
        ),
      );

      final button = find.byType(TextButton);
      expect(button, findsOneWidget);
      expect(clicked, isFalse);

      await tester.tap(button);
      await tester.pumpAndSettle();
      expect(clicked, isTrue);
    });
  });
}
