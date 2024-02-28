import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

import '/core/agent_api_client.dart';

enum LandscapeConfigType { manual, file }

enum FileError { notFound, tooLarge, empty, none }

class LandscapeModel extends ChangeNotifier {
  LandscapeModel(this.client);
  final AgentApiClient client;

  LandscapeConfigType _selected = LandscapeConfigType.manual;

  String _path = '';

  String _fqdn = '';
  String accountName = 'standalone';
  String key = '';

  bool _receivedInput = false;

  bool _fqdnError = false;
  FileError _fileError = FileError.none;

  bool get fqdnError => _fqdnError;
  FileError get fileError => _fileError;

  bool get receivedInput => _receivedInput;

  set fqdn(String value) {
    if (value.isNotEmpty && !value.startsWith('https://')) {
      value = 'https://$value';
    }
    _fqdn = value;
    _receivedInput = true;
    validateFQDN();
  }

  String get fqdn => _fqdn;

  set selected(LandscapeConfigType value) {
    _selected = value;
    _fqdnError = false;
    _fileError = FileError.none;
    validConfig();
    notifyListeners();
  }

  LandscapeConfigType get selected {
    return _selected;
  }

  set path(String value) {
    _path = value;
    _receivedInput = true;
    validatePath();
  }

  String get path => _path;

  bool validateFQDN() {
    _fqdnError = _fqdn.isEmpty || Uri.tryParse(_fqdn) == null;
    notifyListeners();
    return !fqdnError;
  }

  bool validatePath() {
    final file = File(_path);
    final fileStat = file.statSync();
    if (_path.isEmpty) {
      _fileError = FileError.empty;
    } else if (fileStat.size >= 1024 * 1024) {
      _fileError = FileError.tooLarge;
    } else if (fileStat.type == FileSystemEntityType.notFound) {
      _fileError = FileError.notFound;
    } else {
      _fileError = FileError.none;
    }
    notifyListeners();
    return fileError == FileError.none;
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

    return true;
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
