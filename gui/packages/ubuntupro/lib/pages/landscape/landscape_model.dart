import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

import '/core/agent_api_client.dart';

enum LandscapeConfigType { manual, file }

enum FileError { notFound, tooLarge, empty, none }

class LandscapeModel extends ChangeNotifier {
  LandscapeModel(this.client);
  final AgentApiClient client;

  final landscapeURI = Uri.https('ubuntu.com', '/landscape');

  LandscapeConfigType _selected = LandscapeConfigType.manual;

  String _path = '';

  String _fqdn = '';
  String accountName = 'standalone';
  String key = '';

  bool _receivedInput = false;

  bool _fqdnError = false;
  FileError _fileError = FileError.none;

  bool get hasError =>
      fqdnError || fileError != FileError.none || !receivedInput;

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
    switch (selected) {
      case LandscapeConfigType.manual:
        _receivedInput = _fqdn.isNotEmpty;
      case LandscapeConfigType.file:
        _receivedInput = _path.isNotEmpty;
    }
    if (_receivedInput) {
      validConfig();
    }

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
    final uri = Uri.tryParse(_fqdn);
    _fqdnError = _fqdn.isEmpty || uri == null || uri.hasPort;
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
    switch (selected) {
      case LandscapeConfigType.manual:
        return validateFQDN();
      case LandscapeConfigType.file:
        return validatePath();
    }
  }

  Future<bool> applyConfig() async {
    if (!validConfig()) {
      return false;
    }

    switch (selected) {
      case LandscapeConfigType.manual:
        await _applyManualLandscapeConfig();
      case LandscapeConfigType.file:
        await _applyLandscapeConfig();
    }

    return true;
  }

  Future<void> _applyLandscapeConfig() async {
    final file = File(_path);
    final content = await file.readAsString();
    await client.applyLandscapeConfig(content);
  }

  Future<void> _applyManualLandscapeConfig() async {
    final uri = Uri.parse(_fqdn).replace(port: 6554);
    final config = '''
[host]
url = ${uri.authority}
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
    launchUrl(landscapeURI);
  }
}
