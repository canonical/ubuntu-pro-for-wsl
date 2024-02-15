import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:url_launcher/url_launcher_string.dart';
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

    final linkStyle = MarkdownStyleSheet.fromTheme(
      Theme.of(context).copyWith(
        textTheme: DarkStyledLandingPage.textTheme.copyWith(
          bodyMedium: DarkStyledLandingPage.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w100,
            color: YaruColors.warmGrey,
          ),
        ),
      ),
    );

    return DarkStyledLandingPage(
      children: [
        Row(
          children: [
            Icon(
              Icons.check_circle,
              color: YaruColors.dark.success,
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
        MarkdownBody(
          data: caption,
          onTapLink: (_, href, __) => launchUrlString(href!),
          styleSheet: linkStyle,
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
