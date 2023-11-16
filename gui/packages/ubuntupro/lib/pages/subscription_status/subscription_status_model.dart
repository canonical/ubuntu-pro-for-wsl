import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../core/environment.dart';
import '/core/agent_api_client.dart';
import '/core/pro_token.dart';

/// A base class for the view-models that may represent different types of subscriptions and the optional actions they allow.
sealed class SubscriptionStatusModel {
  /// Returns the appropriate view-model subclass based on the SubscriptionInfo that was passed.
  factory SubscriptionStatusModel(
    SubscriptionInfo info,
    AgentApiClient client,
  ) {
    if (!info.immutable) {
      switch (info.whichSubscriptionType()) {
        case SubscriptionType.organization:
          return OrgSubscriptionStatusModel();
        case SubscriptionType.user:
          return UserSubscriptionStatusModel(client);
        case SubscriptionType.microsoftStore:
          return StoreSubscriptionStatusModel(info.productId);
        case SubscriptionType.none:
          return SubscribeNowModel(client);
        case SubscriptionType.notSet:
          throw UnimplementedError('Unknown subscription type');
      }
    }
    return OrgSubscriptionStatusModel();
  }
  SubscriptionStatusModel._();
}

/// Represents an active subscription through Microsoft Store.
/// The only action supported is accessing the user's account web page to manage the subscription to our product.
class StoreSubscriptionStatusModel extends SubscriptionStatusModel {
  @visibleForTesting
  final Uri uri;

  StoreSubscriptionStatusModel(String productID)
      : uri = Uri.https(
          'account.microsoft.com',
          '/services/$productID/details#billing',
        ),
        super._();

  /// Launches the MS account web page where the user can manage the subscription.
  Future<void> launchManagementWebPage() => launchUrl(uri);
}

/// Represents a subscription in which the user manually provided the Pro token.
/// The only action supported is Pro-detaching all instances.
class UserSubscriptionStatusModel extends SubscriptionStatusModel {
  UserSubscriptionStatusModel(this._client) : super._();

  final AgentApiClient _client;

  /// Pro-detach all Ubuntu WSL instances.
  Future<void> detachPro() => _client.applyProToken('');
}

/// Represents a subscription provided by the user's Organization.
/// There is no action supported.
class OrgSubscriptionStatusModel extends SubscriptionStatusModel {
  OrgSubscriptionStatusModel() : super._();
}

class SubscribeNowModel extends SubscriptionStatusModel {
  final AgentApiClient client;
  bool? _isPurchaseAllowed;
  SubscribeNowModel(this.client) : super._();

  Future<SubscriptionInfo> applyProToken(ProToken token) async {
    await client.applyProToken(token.value);
    return client.subscriptionInfo();
  }

  void launchProWebPage() {
    launchUrl(Uri.parse('https://ubuntu.com/pro'));
  }

  /// Triggers a purchase transaction via MS Store.
  /// If the purchase succeeds, this notifies the background agent and returns its [SubscriptionInfo] reply.
  /// Otherwise the purchase status is returned so the UI can give the user some feedback.
  Future<Either<PurchaseStatus, SubscriptionInfo>>
      purchaseSubscription() async {
    try {
      final status = await P4wMsStore().purchaseSubscription(
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

  /// Returns true if the environment variable 'UP4W_ALLOW_STORE_PURCHASE' has been set.
  /// Since this reading won't change during the app lifetime, even if the user changes
  /// it's value from outside, the value is cached so we don't check the environment more than once.
  bool purchaseAllowed() {
    return _isPurchaseAllowed ??= ['true', '1', 'on']
        .contains(Environment()['UP4W_ALLOW_STORE_PURCHASE']?.toLowerCase());
  }
}
