import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import '../../core/either_value_notifier.dart';
import '../../core/pro_token.dart';

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
