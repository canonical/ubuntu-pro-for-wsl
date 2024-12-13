import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/foundation.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';

import '/core/agent_api_client.dart';
import '/core/either_value_notifier.dart';
import '/core/pro_token.dart';

class SubscribeNowModel extends ChangeNotifier {
  final AgentApiClient client;
  P4wMsStore store;

  final _token = ProTokenValue();
  ProTokenValue get token => _token;
  bool get canSubmit => token.valueOrNull != null;

  /// Returns true if the environment variable 'UP4W_ALLOW_STORE_PURCHASE' has been set.
  /// Since this reading won't change during the app lifetime, even if the user changes
  /// it's value from outside, the value is cached so we don't check the environment more than once.
  bool get purchaseAllowed => _isPurchaseAllowed;
  final bool _isPurchaseAllowed;

  SubscribeNowModel(
    this.client, {
    bool isPurchaseAllowed = false,
    P4wMsStore? store,
  })  : _isPurchaseAllowed = isPurchaseAllowed,
        store = store ?? P4wMsStore(),
        super();

  Future<SubscriptionInfo> applyProToken(ProToken token) {
    return client.applyProToken(token.value);
  }

  /// Triggers a purchase transaction via MS Store.
  /// If the purchase succeeds, this notifies the background agent and returns its [SubscriptionInfo] reply.
  /// Otherwise the purchase status is returned so the UI can give the user some feedback.
  Future<Either<PurchaseStatus, SubscriptionInfo>>
      purchaseSubscription() async {
    try {
      final status = await store.purchaseSubscription(
        '9P25B50XMKXT',
      );
      if (status == PurchaseStatus.succeeded) {
        final newInfo = await client.notifyPurchase();
        return newInfo.right();
      }
      return status.left();
    } on Exception catch (err) {
      debugPrint('$err');
      return PurchaseStatus.unknown.left();
    }
  }

  void tokenUpdate(String token) async {
    _token.update(token);
    notifyListeners();
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
