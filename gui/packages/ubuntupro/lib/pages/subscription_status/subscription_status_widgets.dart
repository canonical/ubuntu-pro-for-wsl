import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:yaru/yaru.dart';
import '/pages/widgets/page_widgets.dart';

/// A page content widget built on top of the Dark styled landing page showing the current user active subscription
/// feedback and an optional action button in a column layout.
class SubscriptionStatus extends StatelessWidget {
  const SubscriptionStatus({super.key, this.actionButtons, this.footerLinks});

  /// The optional action button matching the capabilities of the current subscription type.
  final List<Widget>? actionButtons;

  final List<Widget>? footerLinks;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return CenteredPage(
      footer:
          footerLinks != null
              ? Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: footerLinks!,
              )
              : null,
      children: [
        const SizedBox(height: 16.0),
        YaruInfoBox(
          title: Text(lang.ubuntuProEnabled),
          subtitle: Text(lang.ubuntuProEnabledInfo),
          yaruInfoType: YaruInfoType.success,
        ),
        if (actionButtons != null)
          Center(
            child: Padding(
              padding: const EdgeInsets.only(top: 32.0),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [...actionButtons!.map((e) => Flexible(child: e))],
              ),
            ),
          ),
      ],
    );
  }
}
