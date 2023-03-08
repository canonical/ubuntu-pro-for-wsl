import 'p4w_ms_store_platform_interface.dart';

class P4wMsStore {
  Future<String?> getPlatformVersion() {
    return P4wMsStorePlatform.instance.getPlatformVersion();
  }
}
