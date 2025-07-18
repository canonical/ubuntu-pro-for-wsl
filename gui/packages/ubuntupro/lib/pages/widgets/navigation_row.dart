import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';

class NavigationRow extends StatelessWidget {
  const NavigationRow({
    required this.onBack,
    required this.onNext,
    this.backText,
    this.nextText,
    this.showBack = true,
    this.showNext = true,
    this.nextIsAction = true,
    super.key,
  });

  final void Function()? onBack;
  final String? backText;
  final bool showBack;
  final void Function()? onNext;
  final String? nextText;
  final bool showNext;
  final bool nextIsAction;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Row(
      children: [
        if (showBack)
          OutlinedButton(
            onPressed: onBack,
            child: Text(backText ?? lang.buttonBack),
          ),
        if (showNext) ...[
          const Spacer(),
          nextIsAction
              ? ElevatedButton(
                  onPressed: onNext,
                  child: Text(nextText ?? lang.buttonNext),
                )
              : OutlinedButton(
                  onPressed: onNext,
                  child: Text(nextText ?? lang.buttonNext),
                ),
        ],
      ],
    );
  }
}
