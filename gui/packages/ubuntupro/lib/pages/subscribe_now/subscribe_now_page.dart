import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:wizard_router/wizard_router.dart';
import '../widgets/page_widgets.dart';
import '/core/agent_api_client.dart';
import 'subscribe_now_model.dart';
import 'subscribe_now_widgets.dart';

class SubscribeNowPage extends StatelessWidget {
  const SubscribeNowPage({super.key, required this.onSubscriptionUpdate});
  final void Function(SubscriptionInfo) onSubscriptionUpdate;

  @override
  Widget build(BuildContext context) {
    final model = context.watch<SubscribeNowModel>();
    final lang = AppLocalizations.of(context);
    final theme = Theme.of(context);
    return LandingPage(
      children: [
        Text(
          lang.proHeading,
          style:
              theme.textTheme.bodyLarge!.copyWith(fontWeight: FontWeight.w100),
        ),
        const SizedBox(height: 16),
        Row(
          mainAxisAlignment: MainAxisAlignment.start,
          children: [
            Tooltip(
              message: model.purchaseAllowed()
                  ? ''
                  : lang.subscribeNowTooltipDisabled,
              child: ElevatedButton(
                onPressed: model.purchaseAllowed()
                    ? () async {
                        final subs = await model.purchaseSubscription();

                        // Using anything attached to the BuildContext after a suspension point might be tricky.
                        // Better check if it's still mounted in the widget tree.
                        if (!context.mounted) return;

                        subs.fold(
                          ifLeft: (status) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(
                                width: 200.0,
                                behavior: SnackBarBehavior.floating,
                                content: Center(
                                  child: Padding(
                                    padding: const EdgeInsets.symmetric(
                                      vertical: 2.0,
                                      horizontal: 16.0,
                                    ),
                                    child: Text(status.localize(lang)),
                                  ),
                                ),
                              ),
                            );
                          },
                          ifRight: onSubscriptionUpdate,
                        );
                      }
                    : null,
                child: Text(lang.subscribeNow),
              ),
            ),
            const SizedBox(width: 8.0),
            OutlinedButton(
              onPressed: model.launchProWebPage,
              child: Text(lang.about),
            ),
          ],
        ),
        const Padding(
          padding: EdgeInsets.only(top: 16.0, bottom: 24.0),
          child: Divider(thickness: 0.2),
        ),
        ProTokenInputField(
          onApply: (token) {
            model.applyProToken(token).then(onSubscriptionUpdate);
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                width: 400.0,
                behavior: SnackBarBehavior.floating,
                content: Text(
                  lang.applyingProToken(token.value),
                ),
              ),
            );
          },
        ),
      ],
    );
  }

  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    return Provider<SubscribeNowModel>(
      create: (context) => SubscribeNowModel(client),
      child: SubscribeNowPage(
        onSubscriptionUpdate: (info) {
          final src = context.read<ValueNotifier<ConfigSources>>();
          src.value.proSubscription = info;
          Wizard.of(context).next();
        },
      ),
    );
  }
}

extension PurchaseStatusl10n on PurchaseStatus {
  String localize(AppLocalizations lang) {
    switch (this) {
      case PurchaseStatus.succeeded:
        return lang.purchaseStatusSuccess;
      case PurchaseStatus.alreadyPurchased:
        return lang.purchaseStatusAlreadyPurchased;
      case PurchaseStatus.userGaveUp:
        return lang.purchaseStatusUserGaveUp;
      case PurchaseStatus.networkError:
        return lang.purchaseStatusNetwork;
      case PurchaseStatus.serverError:
        return lang.purchaseStatusServer;
      case PurchaseStatus.unknown:
        return lang.purchaseStatusUnknown;
      default:
        throw UnimplementedError(toString());
    }
  }
}
