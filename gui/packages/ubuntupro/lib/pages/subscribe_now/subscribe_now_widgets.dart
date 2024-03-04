import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:yaru/yaru.dart';
import 'package:yaru_icons/yaru_icons.dart';
import 'package:yaru_widgets/yaru_widgets.dart';
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
    return YaruExpandable(
      header: Text(
        lang.tokenInputTitle,
        style: yaruDark.textTheme.bodyMedium!
            .copyWith(fontWeight: FontWeight.w100),
      ),
      expandIcon: Icon(
        YaruIcons.pan_end,
        color: yaruDark.textTheme.bodyMedium!.color,
      ),
      isExpanded: widget.isExpanded,
      child: ValueListenableBuilder(
        valueListenable: _token,
        builder: (context, _, __) => TextField(
          autofocus: false,
          controller: _controller,
          decoration: InputDecoration(
            hintText: lang.tokenInputHint,
            errorText: _token.errorOrNull?.localize(lang),
            counterText: '',
            suffixIcon: ElevatedButton(
              onPressed: canSubmit ? _handleApplyButton : null,
              child: Text(lang.confirm),
            ),
          ),
          onChanged: _token.update,
          onSubmitted: _onSubmitted,
        ),
      ),
    );
  }
}

/// A value-notifier for the [ProToken] with validation.
/// Since we don't want to start the UI with an error due the text field being
/// empty, this stores a nullable [ProToken] object
class ProTokenValue extends EitherValueNotifier<TokenError, ProToken?> {
  ProTokenValue() : super.ok(null);

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
  String localize(AppLocalizations lang) {
    switch (this) {
      case TokenError.empty:
        return lang.tokenErrorEmpty;
      case TokenError.tooShort:
        return lang.tokenErrorTooShort;
      case TokenError.tooLong:
        return lang.tokenErrorTooLong;
      case TokenError.invalidPrefix:
        return lang.tokenErrorInvalidPrefix;
      case TokenError.invalidEncoding:
        return lang.tokenErrorInvalidEncoding;
      default:
        throw UnimplementedError(toString());
    }
  }
}
