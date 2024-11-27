import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:url_launcher/url_launcher_string.dart';
import 'package:wizard_router/wizard_router.dart';

import '/core/agent_api_client.dart';
import '/pages/widgets/page_widgets.dart';

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
    final linkStyle = MarkdownStyleSheet.fromTheme(
      theme.copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyLarge,
        ),
      ),
    );

    return LandingPage(
      children: [
        SizedBox(
          width: 400,
          child: MarkdownBody(
            data: lang.proHeading('[Learn more](https://ubuntu.com/pro)'),
            onTapLink: (_, href, __) => launchUrlString(href!),
            styleSheet: linkStyle,
          ),
        ),
        const SizedBox(height: 16),
        Row(
          mainAxisAlignment: MainAxisAlignment.start,
          children: [
            if (model.purchaseAllowed) ...[
              ElevatedButton(
                onPressed: model.purchaseAllowed
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
                child: Text(lang.getUbuntuPro),
              ),
              const SizedBox(width: 8.0),
            ],
          ],
        ),
        const Padding(
          padding: EdgeInsets.only(top: 16.0, bottom: 24.0),
          child: Divider(thickness: 0.2),
        ),
        ProTokenInputField(
          onApply: (token) {
            model.applyProToken(token).then(onSubscriptionUpdate);
          },
        ),
      ],
    );
  }

  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    final storePurchaseIsAllowed =
        Wizard.of(context).routeData as bool? ?? false;

    return Provider<SubscribeNowModel>(
      create: (context) => SubscribeNowModel(
        client,
        isPurchaseAllowed: storePurchaseIsAllowed,
      ),
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
