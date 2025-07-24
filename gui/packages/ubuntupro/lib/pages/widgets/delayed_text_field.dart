import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';

/// A [TextField] that displays error messages on a delay instead of
/// immediately.
class DelayedTextField extends StatefulWidget {
  const DelayedTextField({
    this.autofocus = false,
    this.enabled = true,
    this.controller,
    this.error,
    this.errorText,
    this.helper,
    this.helperText,
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
  final Widget? helper;
  final String? helperText;
  final String? hintText;
  final List<TextInputFormatter>? inputFormatters;
  final Widget? label;
  final void Function(String)? onChanged;
  final void Function(String)? onSubmitted;

  @override
  State<DelayedTextField> createState() => _DelayedTextField();
}

class _DelayedTextField extends State<DelayedTextField>
    with SingleTickerProviderStateMixin {
  late TimerNotifier debouncer;

  bool showError = false;

  @override
  void initState() {
    super.initState();
    debouncer = TimerNotifier(vsync: this, duration: Durations.medium4);
    debouncer.addListener(() {
      showError = mounted && widget.error != null || widget.errorText != null;
    });
  }

  @override
  void dispose() {
    debouncer.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return ListenableBuilder(
      listenable: debouncer,
      builder: (context, _) {
        return TextField(
          enabled: widget.enabled,
          controller: widget.controller,
          autofocus: widget.autofocus,
          inputFormatters: widget.inputFormatters,
          onChanged: (value) {
            widget.onChanged?.call(value);
            debouncer.stop();
            debouncer.resume();
          },
          onSubmitted: widget.onSubmitted,
          decoration: InputDecoration(
            error: showError ? widget.error : null,
            errorText: showError ? widget.errorText : null,
            label: widget.label,
            helper: widget.helper,
            helperText: widget.helperText,
            hintText: widget.hintText,
            hintMaxLines: 3,
            errorMaxLines: 3,
            helperMaxLines: 3,
            labelStyle: theme.inputDecorationTheme.labelStyle?.copyWith(
              overflow: TextOverflow.ellipsis,
            ),
          ),
        );
      },
    );
  }
}

/// A [ChangeNotifier] that notifies when the provided duration elapses.
class TimerNotifier extends ChangeNotifier {
  TimerNotifier({required this.duration, required this.vsync})
      : assert(duration > Duration.zero, 'Duration must be greater than zero') {
    _ticker = vsync.createTicker(_onTick);
  }

  final TickerProvider vsync;
  final Duration duration;

  Ticker? _ticker;

  @override
  void dispose() {
    _ticker?.dispose();
    super.dispose();
  }

  /// Stops the timer.
  void stop() {
    _ticker?.stop();
  }

  /// Resumes the timer from the last elapsed time.
  void resume() {
    _ticker?.start();
  }

  /// Callback executed on each tick.
  void _onTick(Duration elapsed) {
    if (elapsed >= duration) {
      _ticker?.stop();
      scheduleMicrotask(notifyListeners);
    }
  }
}
