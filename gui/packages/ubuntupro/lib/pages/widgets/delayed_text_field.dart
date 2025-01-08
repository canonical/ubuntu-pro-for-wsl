import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// A [TextField] that displays error messages on a delay instead of
/// immediately.
class DelayedTextField extends StatefulWidget {
  DelayedTextField({
    this.autofocus = false,
    this.enabled = true,
    this.controller,
    this.error,
    this.errorText,
    this.hintText,
    this.inputFormatters,
    this.label,
    this.onChanged,
    this.onSubmitted,
    super.key,
  });

  final bool autofocus;
  final TextEditingController? controller;
  final bool enabled;
  final Widget? error;
  final String? errorText;
  final String? hintText;
  final List<TextInputFormatter>? inputFormatters;
  final Widget? label;
  final void Function(String)? onChanged;
  final void Function(String)? onSubmitted;

  @override
  State<DelayedTextField> createState() => _DelayedTextField();
}

class _DelayedTextField extends State<DelayedTextField> {
  Timer? debounce;

  bool showError = false;

  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    debounce?.cancel();
    super.dispose();
  }

  void onTextChanged() {
    if (debounce?.isActive == true) debounce?.cancel();

    debounce = Timer(Durations.medium4, () {
      setState(() {
        showError = widget.error != null || widget.errorText != null;
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: widget.controller,
      autofocus: widget.autofocus,
      inputFormatters: widget.inputFormatters,
      onChanged: (value) {
        widget.onChanged?.call(value);
        onTextChanged();
      },
      onSubmitted: widget.onSubmitted,
      decoration: InputDecoration(
        error: showError ? widget.error : null,
        errorText: showError ? widget.errorText : null,
        label: widget.label,
      ),
    );
  }
}
