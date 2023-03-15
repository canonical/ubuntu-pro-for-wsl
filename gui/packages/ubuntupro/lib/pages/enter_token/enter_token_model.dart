import '../../core/agent_api_client.dart';
import '../../core/either_value_notifier.dart';
import '../../core/pro_token.dart';

extension TokenErrorl10n on TokenError {
  /// Allows representing the [TokenError] enum as a String.
  // TODO: Replace this by a localizable version when l10n gets setup.
  // String localize(AppLocalizations lang) {
  String localize() {
    switch (this) {
      case TokenError.empty:
        return 'Token cannot be empty';
      case TokenError.tooShort:
        return 'Token is too short';
      case TokenError.tooLong:
        return 'Token is too long';
      case TokenError.invalidPrefix:
        return 'Token prefix is invalid';
      case TokenError.invalidEncoding:
        return 'Token is corrupted';
      default:
        throw UnimplementedError(toString());
    }
  }
}

/// The view-model for the [EnterProTokenPage].
/// Since we don't want to start the UI with an error due the text field being
/// empty, this stores a nullable [ProToken] object
class EnterProTokenModel extends EitherValueNotifier<TokenError, ProToken?> {
  EnterProTokenModel(this.client) : super.ok(null);

  final AgentApiClient client;

  String? get token => valueOrNull?.value;

  bool get hasError => value.isLeft;

  void update(String token) {
    value = ProToken.create(token);
  }

  Future<void> apply() async {
    if (value.isRight) {
      return client.proAttach(valueOrNull!.value);
    }
  }
}
