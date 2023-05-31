import 'p4w_ms_store_platform_interface.dart';

class P4wMsStore {
  Future<PurchaseStatus> purchaseSubscription(String productId) {
    return P4wMsStorePlatform.instance.purchaseSubscription(productId);
  }
}
