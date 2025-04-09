import 'package:flutter/material.dart';
import 'package:yaru/yaru.dart';

class RadioTile<T> extends StatelessWidget {
  const RadioTile({
    required this.value,
    required this.title,
    required this.groupValue,
    required this.onChanged,
    this.subtitle,
    super.key,
  });

  final String title;
  final String? subtitle;
  final T value, groupValue;
  final Function(T?)? onChanged;

  @override
  Widget build(BuildContext context) {
    // Adds a nice visual clue that the tile is selected.
    return YaruBorderContainer(
      border:
          groupValue == value
              ? Border.all(color: Theme.of(context).colorScheme.primary)
              : null,
      // we specify this here since [YaruSelectableContainer] doesn't support a
      // non-selected color
      color:
          groupValue != value
              ? Theme.of(context).colorScheme.onInverseSurface
              : null,
      child: YaruSelectableContainer(
        selected: groupValue == value,
        selectionColor: Theme.of(
          context,
        ).colorScheme.tertiaryContainer.withValues(alpha: 0.8),
        padding: EdgeInsets.zero,
        child: YaruRadioListTile(
          contentPadding: const EdgeInsets.all(6),
          visualDensity: VisualDensity.standard,
          dense: true,
          title: Text(
            title,
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w500),
          ),
          subtitle: subtitle != null ? Text(subtitle!) : null,
          value: value,
          groupValue: groupValue,
          onChanged: onChanged,
        ),
      ),
    );
  }
}
