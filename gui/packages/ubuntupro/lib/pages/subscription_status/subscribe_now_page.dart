import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:yaru/yaru.dart';
import '../widgets/page_widgets.dart';
import 'subscribe_now_widgets.dart';
import 'subscription_status_model.dart';

class SubscribeNowPage extends StatelessWidget {
  const SubscribeNowPage({super.key});

  @override
  Widget build(BuildContext context) {
    final model = context.watch<SubscriptionStatusModel>() as SubscribeNowModel;
    final lang = AppLocalizations.of(context);
    return DarkStyledLandingPage(
      children: [
        Text(
          lang.proHeading,
          style: yaruDark.textTheme.bodyLarge!
              .copyWith(fontWeight: FontWeight.w100),
        ),
        const SizedBox(height: 16),
        Row(
          mainAxisAlignment: MainAxisAlignment.start,
          children: [
            ElevatedButton(
              onPressed: model.purchaseSubscription,
              child: Text(lang.subscribeNow),
            ),
            const Padding(padding: EdgeInsets.only(right: 8.0)),
            FilledButton.tonal(
              onPressed: model.launchProWebPage,
              style: // mimics the secondary button in u.c/pro landing page.
                  Theme.of(context).filledButtonTheme.style!.copyWith(
                        backgroundColor: MaterialStateProperty.all(
                          const Color(0xfff2f2f2),
                        ),
                        foregroundColor: MaterialStateProperty.all(
                          const Color(0xff010101),
                        ),
                      ),
              child: Text(lang.learnMore),
            ),
          ],
        ),
        const Padding(
          padding: EdgeInsets.only(top: 8.0),
          child: Divider(thickness: 0.2),
        ),
        ProTokenInputField(
          onApply: (token) {
            model.applyProToken(token);
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text(
                  AppLocalizations.of(context).applyingProToken(token.value),
                ),
              ),
            );
          },
        ),
      ],
    );
  }
}
