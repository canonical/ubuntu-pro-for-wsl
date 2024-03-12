import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

import '/core/agent_api_client.dart';
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
            actionButtons: [
              if (model.canConfigureLandscape) _landscapeButton(context),
              OutlinedButton(
                onPressed: model.launchManagementWebPage,
                child: Text(lang.manageSubscription),
              ),
            ],
          ),
        UserSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.manuallyManaged(
              '[ubuntu.com/pro/dashboard](https://ubuntu.com/pro/dashboard)',
            ),
            actionButtons: [
              if (model.canConfigureLandscape) _landscapeButton(context),
              FilledButton(
                style: FilledButton.styleFrom(
                  backgroundColor: YaruColors.red,
                ),
                onPressed: () async {
                  await model.detachPro();
                  if (context.mounted) {
                    final wizard = Wizard.of(context);
                    // If more than just this one, we can go back.
                    if (wizard.hasPrevious) {
                      Wizard.of(context).back();
                    } else {
                      // otherwise we need .replace() or .jump(). [small detail of the wizard_router package]
                      await Wizard.of(context).replace();
                    }
                  }
                },
                child: Text(lang.detachPro),
              ),
            ],
          ),
        OrgSubscriptionStatusModel() => SubscriptionStatus(
            caption: lang.orgManaged,
            actionButtons: model.canConfigureLandscape
                ? [_landscapeButton(context)]
                : null,
          ),
      },
    );
  }

  Widget _landscapeButton(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Padding(
      padding: const EdgeInsets.only(right: 16.0),
      child: OutlinedButton(
        onPressed: Wizard.of(context).next,
        child: Text(lang.landscapeConfigureButton),
      ),
    );
  }

  /// Initializes the view-model and inject it in the widget tree so the child page can access it via the BuildContext.
  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    return ProxyProvider<ValueNotifier<ConfigSources>, SubscriptionStatusModel>(
      update: (context, src, _) => SubscriptionStatusModel(src.value, client),
      child: const SubscriptionStatusPage(),
    );
  }
}
