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
    return YaruSelectableContainer(
      selected: groupValue == value,
      selectionColor: Theme.of(context).colorScheme.tertiaryContainer,
      child: YaruRadioListTile(
        contentPadding: EdgeInsets.zero,
        visualDensity: VisualDensity.comfortable,
        dense: true,
        title: Text(title),
        subtitle: subtitle != null ? Text(subtitle!) : null,
        value: value,
        groupValue: groupValue,
        onChanged: onChanged,
      ),
    );
  }
}
