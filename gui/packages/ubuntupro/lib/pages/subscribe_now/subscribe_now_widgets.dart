import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher_string.dart';
import 'package:yaru/yaru.dart';
import '../../core/pro_token.dart';
import 'subscribe_now_model.dart';

/// A validated text field with a submit button that calls the supplied [onApply]
/// callback with the validated Pro Token when the submit button is clicked.
class ProTokenInputField extends StatefulWidget {
  const ProTokenInputField({
    super.key,
    this.isExpanded = false,
  });

  /// Whether the field should be shown expanded or collapsed by default.
  final bool isExpanded;

  /// The icon to be used for the expandable widget, mainly visible for stable tests.
  static const expandIcon = YaruIcons.pan_end;

  @override
  State<ProTokenInputField> createState() => _ProTokenInputFieldState();
}

class _ProTokenInputFieldState extends State<ProTokenInputField> {
  // Only used to clear the text field after submission.
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final theme = Theme.of(context);
    final linkStyle = MarkdownStyleSheet.fromTheme(
      theme.copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyMedium,
        ),
      ),
    );
    final model = context.watch<SubscribeNowModel>();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          lang.tokenInputTitle,
          style: theme.textTheme.titleLarge!,
        ),
        const SizedBox(height: 8),
        MarkdownBody(
          data: lang.tokenInputDescription(
            '[ubuntu.com/pro/dashboard](https://ubuntu.com/pro/dashboard)',
          ),
          onTapLink: (_, href, __) => launchUrlString(href!),
          styleSheet: linkStyle,
        ),
        const SizedBox(height: 8),
        ValueListenableBuilder(
          valueListenable: model.token,
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
                    label: Text(lang.tokenInputHint),
                    error: model.token.errorOrNull?.localize(lang) != null
                        ? Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Text(
                              model.token.errorOrNull!.localize(lang)!,
                              style: theme.textTheme.bodySmall!.copyWith(
                                color: YaruColors.of(context).error,
                              ),
                            ),
                          )
                        : null,
                  ),
                  onChanged: model.tokenUpdate,
                  // onSubmitted: () => model.submit(),
                ),
              ),
            ],
          ),
        ),
      ],
    );
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
