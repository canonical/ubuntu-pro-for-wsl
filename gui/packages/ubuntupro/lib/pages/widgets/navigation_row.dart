import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

class NavigationRow extends StatelessWidget {
  const NavigationRow({
    required this.onBack,
    required this.onNext,
    this.backText,
    this.nextText,
    super.key,
  });

  final void Function()? onBack;
  final String? backText;
  final void Function()? onNext;
  final String? nextText;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        OutlinedButton(
          onPressed: onBack,
          child: Text(backText ?? lang.buttonBack),
        ),
        FilledButton(
          onPressed: onNext,
          child: Text(nextText ?? lang.buttonNext),
        ),
      ],
    );
  }
}
