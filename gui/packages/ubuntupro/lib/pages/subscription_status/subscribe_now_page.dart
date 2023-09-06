import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:yaru/yaru.dart';
import '../widgets/page_widgets.dart';
import 'subscribe_now_widgets.dart';
import 'subscription_status_model.dart';

class SubscribeNowPage extends StatelessWidget {
  const SubscribeNowPage({super.key, required this.onSubscribe});
  final void Function(SubscriptionInfo) onSubscribe;

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
              onPressed: () async {
                final subs = await model.purchaseSubscription();

                // Using anything attached to the BuildContext after a suspension point might be tricky.
                // Better check if it's still mounted in the widget tree.
                if (!context.mounted) return;

                subs.fold(
                  ifLeft: (status) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Center(child: Text(status.localize(lang))),
                      ),
                    );
                  },
                  ifRight: onSubscribe,
                );
              },
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
                  lang.applyingProToken(token.value),
                ),
              ),
            );
          },
        ),
      ],
    );
  }
}

extension PurchaseStatusl10n on PurchaseStatus {
  String localize(AppLocalizations lang) {
    switch (this) {
      case PurchaseStatus.succeeded:
        return lang.purchaseStatusSuccess;
      case PurchaseStatus.alreadyPurchased:
        return lang.purchaseStatusAlreadyPurchased;
      case PurchaseStatus.userGaveUp:
        return lang.purchaseStatusUserGaveUp;
      case PurchaseStatus.networkError:
        return lang.purchaseStatusNetwork;
      case PurchaseStatus.serverError:
        return lang.purchaseStatusServer;
      case PurchaseStatus.unknown:
        return lang.purchaseStatusUnknown;
    }
  }
}
