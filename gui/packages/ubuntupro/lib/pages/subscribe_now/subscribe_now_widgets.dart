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
    required this.onSubmit,
    this.controller,
    this.isExpanded = false,
  });

  /// Whether the field should be shown expanded or collapsed by default.
  final bool isExpanded;
  final void Function()? onSubmit;
  final TextEditingController? controller;

  @override
  State<ProTokenInputField> createState() => _ProTokenInputFieldState();
}

class _ProTokenInputFieldState extends State<ProTokenInputField> {
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
        TextField(
          inputFormatters: [
            // This ignores all sorts of (Unicode) whitespaces (not only at the ends).
            FilteringTextInputFormatter.deny(RegExp(r'\s')),
          ],
          autofocus: false,
          controller: widget.controller,
          decoration: InputDecoration(
            label: Text(lang.tokenInputHint),
            error: model.token.error?.localize(lang) != null
                ? Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      model.token.error!.localize(lang)!,
                      style: theme.textTheme.bodySmall!.copyWith(
                        color: YaruColors.of(context).error,
                      ),
                    ),
                  )
                : null,
          ),
          onChanged: model.tokenUpdate,
          onSubmitted: (_) => widget.onSubmit?.call(),
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
