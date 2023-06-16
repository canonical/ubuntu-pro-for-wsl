import 'package:p4w_ms_store/p4w_ms_store.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../core/agent_api_client.dart';
import '../../core/pro_token.dart';

class SubscribeNowModel {
  final AgentApiClient client;
  SubscribeNowModel(this.client);

  Future<void> applyProToken(ProToken token) {
    return client.applyProToken(token.value);
  }

  void launchProWebPage() {
    launchUrl(Uri.parse('https://ubuntu.com/pro'));
  }

// TODO: Communicate this with the agent's UI Service to
// - Get the product ID
// - Notify it of the result of the purchase
// - Display errors
  Future<void> purchaseSubscription() async {
    await P4wMsStore().purchaseSubscription('9P25B50XMKXT');
  }
}
