import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_widgets.dart';
import 'package:yaru_test/yaru_test.dart';

import '../../utils/build_multiprovider_app.dart';

void main() {
  group('subscription status', () {
    const footerText = 'my footer';
    const buttonText = 'my button';

    testWidgets('footer', (tester) async {
      var clicked = false;
      await tester.pumpWidget(
        buildSingleRouteMultiProviderApp(
          child: SubscriptionStatus(
            footerLinks: [
              TextButton(
                onPressed: () => clicked = true,
                child: const Text(footerText),
              ),
            ],
          ),
        ),
      );

      final button = find.button(footerText);
      expect(button, findsOneWidget);

      expect(clicked, isFalse);
      await tester.tap(button);
      await tester.pumpAndSettle();
      expect(clicked, isTrue);
    });

    testWidgets('action button', (tester) async {
      var clicked = false;
      await tester.pumpWidget(
        buildSingleRouteMultiProviderApp(
          child: SubscriptionStatus(
            actionButtons: [
              TextButton(
                onPressed: () => clicked = true,
                child: const Text(buttonText),
              ),
            ],
          ),
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
