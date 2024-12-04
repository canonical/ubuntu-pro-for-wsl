import 'package:flutter/material.dart';

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
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        OutlinedButton(onPressed: onBack, child: Text(backText ?? 'Back')),
        FilledButton(onPressed: onNext, child: Text(nextText ?? 'Next')),
      ],
    );
  }
}
