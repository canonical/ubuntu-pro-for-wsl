import 'dart:io';

import 'package:flutter/foundation.dart';

import '/core/agent_api_client.dart';

enum LandscapeConfigType { manual, file }

enum FileError { notFound, tooLarge, empty, none }

class LandscapeModel extends ChangeNotifier {
  LandscapeModel(this.client);
  final AgentApiClient client;

  static const landscapeSaas = 'landscape.canonical.com';
  static const standalone = 'standalone';
  final landscapeURI = Uri.https('ubuntu.com', '/landscape');

  LandscapeConfigType _selected = LandscapeConfigType.manual;

  String _path = '';

  String _fqdn = '';
  String _accountName = '';
  String key = '';

  bool _receivedInput = false;

  bool _fqdnError = false;
  bool _accountNameError = false;
  bool get accountNameError => _accountNameError;
  bool get canEnterAccountName => _fqdn.endsWith(landscapeSaas);

  FileError _fileError = FileError.none;

  bool get hasError =>
      fqdnError ||
      accountNameError ||
      fileError != FileError.none ||
      !receivedInput;

  bool get fqdnError => _fqdnError;
  FileError get fileError => _fileError;

  bool get receivedInput => _receivedInput;

  set fqdn(String value) {
    if (value.isNotEmpty && !value.startsWith('https://')) {
      value = 'https://$value';
    }
    _fqdn = value;
    if (!_fqdn.endsWith(landscapeSaas)) {
      _accountName = standalone;
      _accountNameError = false;
    } else {
      _accountName = '';
    }
    _receivedInput = true;
    validateFQDN();
    notifyListeners();
  }

  String get fqdn => _fqdn;

  set accountName(String value) {
    // Only accept account names if it's for Landscape SaaS.
    if (_fqdn.endsWith(landscapeSaas)) {
      _accountName = value;
    }
    validateAccountName();
    notifyListeners();
  }

  String get accountName => _accountName;

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

  bool validateAccountName() {
    if (_fqdn.endsWith(landscapeSaas)) {
      _accountNameError = _accountName.isEmpty || _accountName == standalone;
    } else {
      _accountNameError = _accountName != standalone;
    }
    return !accountNameError;
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
        return validateFQDN() && validateAccountName();
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
}
