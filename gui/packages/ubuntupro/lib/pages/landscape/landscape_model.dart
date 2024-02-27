import 'dart:io';

import 'package:agentapi/agentapi.dart';
import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

import '/core/agent_api_client.dart';

enum LandscapeConfigType { manual, file }

class LandscapeModel extends ChangeNotifier {
  LandscapeModel(this.client);
  final AgentApiClient client;

  LandscapeConfigType _selected = LandscapeConfigType.manual;

  String _path = '';

  String _fqdn = '';
  String accountName = 'standalone';
  String key = '';

  bool fqdnError = false;
  bool fileError = false;

  set fqdn(String value) {
    if (!value.startsWith('https://')) {
      value = 'https://$value';
    }
    _fqdn = value;
    validateFQDN();
  }

  set selected(LandscapeConfigType value) {
    _selected = value;
    notifyListeners();
  }

  LandscapeConfigType get selected {
    return _selected;
  }

  set path(String value) {
    _path = value;
    validatePath();
  }

  bool validateFQDN() {
    fqdnError = _fqdn.isEmpty || Uri.tryParse(_fqdn) == null;
    notifyListeners();
    return !fqdnError;
  }

  bool validatePath() {
    fileError = _path.isEmpty || !File(_path).existsSync();
    notifyListeners();
    return !fileError;
  }

  bool validConfig() {
    var valid = false;
    switch (selected) {
      case LandscapeConfigType.manual:
        valid = validateFQDN();
      case LandscapeConfigType.file:
        valid = validatePath();
      default:
        throw UnimplementedError('Unknown configuration type');
    }

    return valid;
  }

  Future<bool> applyConfig() async {
    if (!validConfig()) {
      return false;
    }

    switch (selected) {
      case LandscapeConfigType.manual:
        await applyManualLandscapeConfig();
      case LandscapeConfigType.file:
        await applyLandscapeConfig();
      default:
        throw UnimplementedError('Unknown configuration status');
    }

    final subscriptionInfo = await client.subscriptionInfo();
    final subscriptionType = subscriptionInfo.whichSubscriptionType();
    return ![
      SubscriptionInfo_SubscriptionType.none,
      SubscriptionInfo_SubscriptionType.notSet,
    ].contains(subscriptionType);
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
