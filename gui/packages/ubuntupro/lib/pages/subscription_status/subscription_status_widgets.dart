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
    this.actionButtons,
  });

  /// The caption to render below the active subscription subtitle.
  final String caption;

  /// The optional action button matching the capabilities of the current subscription type.
  final List<Widget>? actionButtons;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    final theme = Theme.of(context);
    final linkStyle = MarkdownStyleSheet.fromTheme(
      theme.copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w100,
          ),
        ),
      ),
    ).copyWith(
      a: const TextStyle(
        decoration: TextDecoration.underline,
      ),
    );

    return LandingPage(
      centered: true,
      children: [
        const SizedBox(height: 16.0),
        YaruInfoBox(
          title: Text(lang.subscriptionIsActive),
          subtitle: MarkdownBody(
            data: caption,
            onTapLink: (_, href, __) => launchUrlString(href!),
            styleSheet: linkStyle,
          ),
          yaruInfoType: YaruInfoType.success,
        ),
        if (actionButtons != null)
          Center(
            child: Padding(
              padding: const EdgeInsets.only(top: 32.0),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  ...actionButtons!.map((e) => Flexible(child: e)),
                ],
              ),
            ),
          ),
      ],
    );
  }
}
