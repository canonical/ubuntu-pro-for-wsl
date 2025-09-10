import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:ubuntu_logger/ubuntu_logger.dart';
import 'package:url_launcher/url_launcher.dart';
import '/core/agent_api_client.dart';

final _log = Logger('subscription_status');

/// A base class for the view-models that may represent different types of subscriptions and the optional actions they allow.
sealed class SubscriptionStatusModel {
  /// Returns the appropriate view-model subclass based on the subscription source type that was passed.
  factory SubscriptionStatusModel(
    ConfigSources src,
    AgentApiClient client, {
    bool canConfigureLandscape = false,
  }) {
    // Enforce this business logic here, as defaults may change in the future:
    //   - Org-managed Landscape configurations don't allow user changes via GUI.
    if (src.landscapeSource.hasOrganization()) {
      canConfigureLandscape = false;
    }

    final info = src.proSubscription;
    switch (info.whichSubscriptionType()) {
      case SubscriptionType.organization:
        return OrgSubscriptionStatusModel()
          .._canConfigureLandscape = canConfigureLandscape;
      case SubscriptionType.user:
        return UserSubscriptionStatusModel(client)
          .._canConfigureLandscape = canConfigureLandscape;
      case SubscriptionType.microsoftStore:
        return StoreSubscriptionStatusModel(info.productId)
          .._canConfigureLandscape = canConfigureLandscape;
      case SubscriptionType.none:
      case SubscriptionType.notSet:
        throw UnimplementedError('Unknown subscription type');
    }
  }

  SubscriptionStatusModel._();

  /// Tells whether we can invoke the Landscape configuration page or not.
  bool get canConfigureLandscape => _canConfigureLandscape;
  bool _canConfigureLandscape = false;
}

/// Represents an active subscription through Microsoft Store.
/// The only action supported is accessing the user's account web page to manage the subscription to our product.
class StoreSubscriptionStatusModel extends SubscriptionStatusModel {
  @visibleForTesting
  final Uri uri;

  StoreSubscriptionStatusModel(String productID)
      : uri = Uri.https(
          'account.microsoft.com',
          '/services/${productID.toLowerCase()}/details#billing',
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
  Future<SubscriptionInfo> detachPro() {
    _log.info('Detach Pro requested.');
    return _client.applyProToken('');
  }
}

/// Represents a subscription provided by the user's Organization.
/// There is no action supported.
class OrgSubscriptionStatusModel extends SubscriptionStatusModel {
  OrgSubscriptionStatusModel() : super._();
}
