import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';

import '/core/agent_api_client.dart';
import 'subscribe_now_page.dart';
import 'subscription_status_model.dart';
import 'subscription_status_widgets.dart';

/// The page to be shown when we have an active Pro subscription. The exact contents will match the type of the subscription
/// (i.e., whether the Pro token was set manually, through MS Store or provided by the user's Organization)
class SubscriptionStatusPage extends StatelessWidget {
  const SubscriptionStatusPage({super.key});

  // TODO: Replace the constants below with YaruColors.dark.link and YaruColors.dark.error once we release yaru v0.9 or v1.0.0.
  @override
  Widget build(BuildContext context) {
    final model = context.watch<SubscriptionStatusModel>();
    final lang = AppLocalizations.of(context);

    return AnimatedSwitcher(
      duration: const Duration(milliseconds: 700),
      child: switch (model) {
        StoreSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.storeManaged,
            actionButton: TextButton(
              onPressed: model.launchManagementWebPage,
              style: TextButton.styleFrom(
                foregroundColor: const Color(0xFF0094FF),
              ),
              child: Text(lang.manageSubscription),
            ),
          ),
        UserSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.manuallyManaged,
            actionButton: FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: const Color(0xFFE86581),
              ),
              onPressed: model.detachPro,
              child: Text(lang.detachPro),
            ),
          ),
        OrgSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.orgManaged,
          ),
        SubscribeNowModel() => SubscribeNowPage(
            onSubscribed: (info) =>
                context.read<ValueNotifier<SubscriptionInfo>>().value = info,
          ),
      },
    );
  }

  /// Initializes the view-model and inject it in the widget tree so the child page can access it via the BuildContext.
  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    return ProxyProvider<ValueNotifier<SubscriptionInfo>,
        SubscriptionStatusModel>(
      update: (context, subscriptionInfo, _) =>
          SubscriptionStatusModel(subscriptionInfo.value, client),
      child: const SubscriptionStatusPage(),
    );
  }
}
