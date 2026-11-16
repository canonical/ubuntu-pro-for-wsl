import 'package:agentapi/agentapi.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:url_launcher/url_launcher_string.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

import '/core/agent_api_client.dart';
import '/l10n/app_localizations.dart';
import '/pages/widgets/navigation_row.dart';
import '/pages/widgets/page_widgets.dart';
import 'subscribe_now_model.dart';
import 'subscribe_now_widgets.dart';

class SubscribeNowPage extends StatefulWidget {
  const SubscribeNowPage({super.key, required this.onSubscriptionUpdate});

  final void Function(SubscriptionInfo) onSubscriptionUpdate;

  @override
  State<SubscribeNowPage> createState() => _SubscribeNowPageState();

  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    final storePurchaseIsAllowed =
        Wizard.of(context).routeData as bool? ?? false;

    return ChangeNotifierProvider<SubscribeNowModel>(
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

class _SubscribeNowPageState extends State<SubscribeNowPage> {
  final controller = TextEditingController();

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final model = context.watch<SubscribeNowModel>();
    final lang = AppLocalizations.of(context);

    return ColumnPage(
      left: [Text(lang.proHeading)],
      right: [
        ProTokenInputField(
          onSubmit: model.canSubmit ? () => trySubmit(model) : null,
          controller: controller,
        ),
        const SizedBox(height: 12.0),
        YaruInfoBox(
          yaruInfoType: YaruInfoType.information,
          title: Text(lang.dontHaveToken),
          child: MarkdownBody(
            data:
                '[${lang.getYourToken}](https://ubuntu.com/pro). ${lang.getYourTokenContinuation}',
            styleSheet: MarkdownStyleSheet(
              a: TextStyle(
                color: YaruColors.of(context).link,
                decoration: TextDecoration.underline,
              ),
            ),
            onTapLink: (_, href, __) async {
              if (!model.purchaseAllowed) {
                await launchUrlString(href!);
                return;
              }
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
                ifRight: widget.onSubscriptionUpdate,
              );
            },
          ),
        ),
      ],
      navigationRow: NavigationRow(
        showBack: false,
        onBack: null,
        onNext: model.canSubmit ? () => trySubmit(model) : null,
        next: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16.0),
          child: Text(lang.attach),
        ),
      ),
    );
  }

  void trySubmit(SubscribeNowModel model) {
    model.applyProToken(model.token!).then(widget.onSubscriptionUpdate);
    model.clearToken();
    controller.clear();
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
    }
  }
}
