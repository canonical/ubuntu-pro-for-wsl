import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

import '/core/agent_api_client.dart';

class LandscapeModel extends ChangeNotifier {
  LandscapeModel(this.client);
  final AgentApiClient client;

  String _path = '';

  String _fqdn = '';
  String accountName = 'standalone';
  String key = '';

  bool fqdnError = false;

  set fqdn(String value) {
    if (!value.startsWith('https://')) {
      value = 'https://$value';
    }
    fqdnError = Uri.tryParse(value) == null;
    if (!fqdnError) {
      _fqdn = value;
    }
    notifyListeners();
  }

  set path(String value) {
    _path = value;
    notifyListeners();
  }

  Future<void> applyLandscapeConfig() async {
    final file = File(_path);
    final content = await file.readAsString();
    await client.applyLandscapeConfig(content);
  }

  Future<void> applyManualLandscapeConfig() async {
    final config = '''
[host]
url = $_fqdn:6554
[client]
account_name = $accountName
registration_key = $key
url = $_fqdn/message-system
log_level = debug
ping_url = $_fqdn/ping
''';
    await client.applyLandscapeConfig(config);
  }

  void launchLandscapeWebPage() {
    launchUrl(Uri.parse('https://ubuntu.com/landscape'));
  }
}
