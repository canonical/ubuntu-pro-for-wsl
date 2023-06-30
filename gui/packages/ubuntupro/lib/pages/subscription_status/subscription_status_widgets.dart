import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:yaru/yaru.dart';
import '../widgets/page_widgets.dart';

/// A page content widget built on top of the Dark styled landing page showing the current user active subscription
/// feedback and an optional action button in a column layout.
class SubscriptionStatus extends StatelessWidget {
  const SubscriptionStatus({
    super.key,
    required this.caption,
    this.actionButton,
  });

  /// The caption to render below the active subscription subtitle.
  final String caption;

  /// The optional action button matching the capabilities of the current subscription type.
  final Widget? actionButton;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return DarkStyledLandingPage(
      children: [
        Row(
          children: [
            const Icon(
              Icons.check_circle,
            ),
            const SizedBox(width: 8.0),
            Text(
              lang.subscriptionIsActive,
              style: DarkStyledLandingPage.textTheme.bodyLarge!
                  .copyWith(fontWeight: FontWeight.w100),
            ),
          ],
        ),
        const SizedBox(height: 16.0),
        Text(
          caption,
          style: DarkStyledLandingPage.textTheme.bodyMedium!.copyWith(
            fontWeight: FontWeight.w100,
            color: YaruColors.warmGrey,
          ),
        ),
        if (actionButton != null)
          Padding(
            padding: const EdgeInsets.only(top: 32.0),
            child: actionButton!,
          ),
      ],
    );
  }
}
