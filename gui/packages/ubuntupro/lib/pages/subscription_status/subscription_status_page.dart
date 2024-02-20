import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:yaru/yaru.dart';

import '/core/agent_api_client.dart';
import 'subscribe_now_page.dart';
import 'subscription_status_model.dart';
import 'subscription_status_widgets.dart';

/// The page to be shown when we have an active Pro subscription. The exact contents will match the type of the subscription
/// (i.e., whether the Pro token was set manually, through MS Store or provided by the user's Organization)
class SubscriptionStatusPage extends StatelessWidget {
  const SubscriptionStatusPage({super.key});

  @override
  Widget build(BuildContext context) {
    final model = context.watch<SubscriptionStatusModel>();
    final lang = AppLocalizations.of(context);

    return AnimatedSwitcher(
      duration: const Duration(milliseconds: 700),
      child: switch (model) {
        StoreSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.storeManaged,
            actionButton: OutlinedButton(
              onPressed: model.launchManagementWebPage,
              child: Text(lang.manageSubscription),
            ),
          ),
        UserSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.manuallyManaged(
              '[ubuntu.com/pro/dashboard](https://ubuntu.com/pro/dashboard)',
            ),
            actionButton: FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: YaruColors.red,
              ),
              onPressed: () async {
                context.read<ValueNotifier<SubscriptionInfo>>().value =
                    await model.detachPro();
              },
              child: Text(lang.detachPro),
            ),
          ),
        OrgSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.orgManaged,
          ),
        SubscribeNowModel() => SubscribeNowPage(
            onSubscriptionUpdate: (info) =>
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
