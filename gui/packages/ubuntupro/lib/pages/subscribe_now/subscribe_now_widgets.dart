import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:yaru/yaru.dart';
import '../../constants.dart';
import '../../core/either_value_notifier.dart';
import '../../core/pro_token.dart';

/// A validated text field with a submit button that calls the supplied [onApply]
/// callback with the validated Pro Token when the submit button is clicked.
class ProTokenInputField extends StatefulWidget {
  const ProTokenInputField({
    super.key,
    this.isExpanded = false,
    required this.onApply,
  });

  /// A function to be called when the user submits a valid Pro Token.
  final void Function(ProToken token) onApply;

  /// Whether the field should be shown expanded or collapsed by default.
  final bool isExpanded;

  /// The icon to be used for the expandable widget, mainly visible for stable tests.
  static const expandIcon = YaruIcons.pan_end;

  @override
  State<ProTokenInputField> createState() => _ProTokenInputFieldState();
}

class _ProTokenInputFieldState extends State<ProTokenInputField> {
  // The value notifier behind this widget state.
  final _token = ProTokenValue();
  // Only used to clear the text field after submission.
  final _controller = TextEditingController();
  // Whether the submit action and the apply button should be enabled.
  bool get canSubmit => _token.valueOrNull != null;

  // This is never called if the token is invalid.
  void _handleApplyButton() {
    widget.onApply(_token.valueOrNull!);
    _token.clear();
    _controller.clear();
  }

  void _onSubmitted(String value) {
    if (canSubmit) {
      return _handleApplyButton();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _token.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          lang.tokenInputTitle,
          style:
              theme.textTheme.bodyMedium!.copyWith(fontWeight: FontWeight.w100),
        ),
        const SizedBox(
          height: 8,
        ),
        ValueListenableBuilder(
          valueListenable: _token,
          builder: (context, _, __) => Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: TextField(
                  inputFormatters: [
                    // This ignores all sorts of (Unicode) whitespaces (not only at the ends).
                    FilteringTextInputFormatter.deny(RegExp(r'\s')),
                  ],
                  autofocus: false,
                  controller: _controller,
                  decoration: InputDecoration(
                    hintText: lang.tokenInputHint,
                    error: _token.errorOrNull?.localize(lang) != null
                        ? Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Row(
                              children: [
                                const Icon(
                                  Icons.cancel,
                                  color: Colors.red,
                                  size: 16.0,
                                ),
                                const SizedBox(width: 4),
                                Text(
                                  _token.errorOrNull!.localize(lang)!,
                                  style: theme.textTheme.bodySmall!
                                      .copyWith(color: Colors.redAccent),
                                ),
                              ],
                            ),
                          )
                        : null,
                    helper: _token.valueOrNull != null
                        ? Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Row(
                              children: [
                                const Icon(
                                  Icons.check_circle,
                                  color: kConfirmColor,
                                  size: 16.0,
                                ),
                                const SizedBox(width: 4),
                                Text(
                                  lang.tokenValid,
                                  style: theme.textTheme.bodySmall!
                                      .copyWith(color: Colors.green),
                                ),
                              ],
                            ),
                          )
                        : null,
                  ),
                  onChanged: _token.update,
                  onSubmitted: _onSubmitted,
                ),
              ),
              const SizedBox(
                width: 8.0,
              ),
              ElevatedButton(
                onPressed: canSubmit ? _handleApplyButton : null,
                child: Text(lang.attach),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// A value-notifier for the [ProToken] with validation.
class ProTokenValue extends EitherValueNotifier<TokenError, ProToken?> {
  ProTokenValue() : super.err(TokenError.empty);

  String? get token => valueOrNull?.value;

  bool get hasError => value.isLeft;

  void update(String token) {
    value = ProToken.create(token);
  }

  void clear() {
    value = const Right<TokenError, ProToken?>(null);
  }
}

extension TokenErrorl10n on TokenError {
  /// Allows representing the [TokenError] enum as a String.
  String? localize(AppLocalizations lang) {
    switch (this) {
      case TokenError.empty:
        // empty cannot be submitted, but we don't need to display an error to
        // the user, just return to original state
        return null;
      case TokenError.invalid:
        return lang.tokenErrorInvalid;
    }
  }
}
