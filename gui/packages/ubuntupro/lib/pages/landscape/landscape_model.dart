import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart' show ChangeNotifier;
import 'package:grpc/grpc.dart' show GrpcError, StatusCode;
import 'package:pkcs7/pkcs7.dart';
import 'package:ubuntu_logger/ubuntu_logger.dart';

import '/core/agent_api_client.dart';

const landscapeSaasFQDN = 'landscape.canonical.com';
const standaloneAN = 'standalone';

final _log = Logger('landscape');

/// The view model for the Landscape configuration page.
/// This class is responsible for managing the state of the Landscape configuration form, including its subforms
/// and submit the active form data when complete, disregarding the inactive ones.
/// Data validation is delegated to the subform data models.
class LandscapeModel extends ChangeNotifier {
  /// The client connection to the background agent.
  final AgentApiClient client;

  LandscapeModel(this.client);

  /// The URL to be shown in the UI.
  static final landscapeURI = Uri.https('ubuntu.com', '/landscape');

  /// Whether the current form is complete (ready to be submitted).
  bool get isComplete => _active.isComplete;

  bool get accountNameIsRequired =>
      configType == LandscapeConfigType.manual &&
      manual.fqdn.endsWith(landscapeSaasFQDN);

  /// Whether we are waiting on agent's response after submitting a configuration
  bool _waiting = false;
  bool get isWaiting => _waiting;

  /// The current configuration type, allowing the UI to show the correct form.
  LandscapeConfigType get configType => _current;
  LandscapeConfigType _current = LandscapeConfigType.manual;

  // The active configuration form data, a shortcut to reduce some switch statements
  // and avoid relying on ducktyping when serializing the config or checking for completeness.
  late LandscapeConfig _active = manual;

  /// The configuration form data for the manual configuration.
  final LandscapeManualConfig manual = LandscapeManualConfig();

  /// The configuration form data for the custom configuration.
  final LandscapeCustomConfig custom = LandscapeCustomConfig();

  /// Allows the UI to inform the selected configuration type.
  void setConfigType(LandscapeConfigType? value) {
    if (value == null) return;
    _current = value;
    switch (configType) {
      case LandscapeConfigType.manual:
        _active = manual;
      case LandscapeConfigType.custom:
        _active = custom;
    }

    notifyListeners();
  }

  /// Sets the registration key for the manual configurations.
  void setManualRegistrationKey(String? registrationKey) {
    assert(_active is LandscapeManualConfig);
    if (registrationKey == null) return;
    manual.registrationKey = registrationKey;
    notifyListeners();
  }

  /// Sets (and validates) the FQDN for the manual configuration.
  void setFqdn(String? fqdn) {
    assert(_active is LandscapeManualConfig);
    if (fqdn == null) return;
    manual.fqdn = fqdn;
    notifyListeners();
  }

  /// Sets (and validates) the FQDN for the manual configuration.
  void setAccountName(String? account) {
    assert(_active is LandscapeManualConfig);
    if (account == null) return;
    manual.accountName = account;
    notifyListeners();
  }

  /// Sets (and validates) the SSL key path for the manual configuration.
  void setSslKeyPath(String? sslKeyPath) {
    assert(_active is LandscapeManualConfig);
    if (sslKeyPath == null) return;
    manual.sslKeyPath = sslKeyPath;
    notifyListeners();
  }

  /// Sets (and validates) the custom configuration path.
  void setCustomConfigPath(String? configPath) {
    assert(_active is LandscapeCustomConfig);
    if (configPath == null) return;
    custom.configPath = configPath;
    notifyListeners();
  }

  /// Translates and submits the active configuration data to the background agent, returning an error message if any.
  Future<GrpcError> applyConfig() async {
    assert(_active.isComplete);
    final config = _active.config();
    assert(config != null);
    var err = const GrpcError.ok();

    switch (_active) {
      case final LandscapeManualConfig c:
        _log.debug('Submitting manual configuration for server ${c.fqdn}');
      case final LandscapeCustomConfig c:
        _log.debug('Submitting custom configuration from ${c.configPath}');
    }

    try {
      _waiting = true;
      notifyListeners();
      await client.applyLandscapeConfig(config!);
    } on GrpcError catch (e) {
      err = e;
    } on Exception catch (e) {
      err = GrpcError.unknown(e.toString());
    }
    _waiting = false;
    notifyListeners();

    if (err.code != StatusCode.ok) {
      _log.debug(
        'Failed to submit the Landscape configuration: ${err.message}',
      );
    }
    return err;
  }
}

/// The different types of Landscape configurations, modelled as an enum to make it easy on the UI side to switch between them.
enum LandscapeConfigType { manual, custom }

/// The alternative errors we could encounter when validating file paths submitted as part of any subform data.
enum FileError {
  notFound,
  tooLarge,
  emptyPath,
  dir,
  emptyFile,
  none,
  invalidFormat,
}

enum FqdnError { invalid, none }

enum AccountNameError { invalid, none }

const validCertExtensions = ['cer', 'crt', 'der', 'pem'];

/// The base class for the closed set of Landscape configuration form types.
sealed class LandscapeConfig {
  /// Whether the form has enough data for submission.
  bool get isComplete;

  /// The raw representation of the configuration (as expected by the background agent).
  String? config();
}

/// The manual configuration form data: only the FQDN is mandatory, and must not
/// match landscape.canonical.com.
class LandscapeManualConfig extends LandscapeConfig {
  Uri? _fqdn;
  String get fqdn => _fqdn?.toString() ?? '';
  FqdnError _fqdnError = FqdnError.none;
  FqdnError get fqdnError => _fqdnError;

  String _accountName = standaloneAN;
  AccountNameError _accountNameError = AccountNameError.none;
  bool get hasAccountNameError => _accountNameError != AccountNameError.none;
  AccountNameError get accountNameError => _accountNameError;
  set accountName(String value) {
    _enforceAccountNameForHost(value, _fqdn?.host);
  }

  void _enforceAccountNameForHost(String account, String? host) {
    if (host == landscapeSaasFQDN) {
      if (account == standaloneAN || account.isEmpty) {
        _accountNameError = AccountNameError.invalid;
        return;
      }
      _accountNameError = AccountNameError.none;
      _accountName = account;
    } else {
      // If not using Landscape SaaS, enforce the standalone account name.
      _accountName = standaloneAN;
      _accountNameError = AccountNameError.none;
    }
  }

  String registrationKey = '';

  String _sslKeyPath = '';
  String get sslKeyPath => _sslKeyPath;

  FileError _fileError = FileError.none;
  FileError get fileError => _fileError;

  // FQDN must be a valid URL (without an explicit port) and must not be the Landscape SaaS URL.
  bool _validateFQDN(Uri? uri) {
    _fqdnError = FqdnError.none;

    if (uri == null || uri.host.isEmpty) {
      _fqdnError = FqdnError.invalid;
    }
    return _fqdnError == FqdnError.none;
  }

  /// Ensure the FQDN is a valid URL, enforcing https without requiring the user to type it.
  set fqdn(String value) {
    final url = _sanitizeFqdn(value);

    if (_validateFQDN(url)) {
      _fqdn = url;
    }

    _enforceAccountNameForHost(_accountName, _fqdn?.host);
  }

  Uri? _sanitizeFqdn(String value) {
    // If the value is a valid IP address, we can return it as is.
    final addr = InternetAddress.tryParse(value);
    if (addr != null) {
      return Uri(host: addr.host);
    }
    final url = Uri.tryParse(value);
    if (url == null) {
      return null;
    }
    // URL being parsed with a single segment, no authority (user@host:port), no queries might be a special case:
    if (url.pathSegments.length == 1 &&
        url.authority.isEmpty &&
        !url.hasFragment &&
        !url.hasQuery) {
      // A single string is parsed as a single segment path, but the user wanted it to be the FQDN instead,
      // so let's move the value to the right field.
      if (!url.hasScheme) {
        return url.replace(
          host: url.path,
          path: '',
        );
      }
      // If we have scheme it might be because the user followed Landscape documentation that advertises
      // the [host].url configuration field to be like `host:port`.
      // That would be parsed by URL libraries as `scheme:path`, so we need to ensure that's the case and
      // fix the URL before further processing.
      final port = int.tryParse(url.path);
      if (port != null && port > 0 && port < 65536) {
        return url.replace(
          host: url.scheme,
          port: port,
          path: '',
          scheme: '',
        );
      }
    }
    return url;
  }

  // If a path is provided, then it must exist and be a non-empty file.
  bool _validatePath(String path) {
    // Empty paths are allowed, since this field is optional.
    if (path.isEmpty) {
      _fileError = FileError.none;
      return true;
    }

    final file = File(path);
    final fileStat = file.statSync();

    if (fileStat.type == FileSystemEntityType.notFound) {
      _fileError = FileError.notFound;
    } else if (fileStat.type == FileSystemEntityType.directory) {
      _fileError = FileError.dir;
    } else if (fileStat.size == 0) {
      _fileError = FileError.emptyFile;
    } else if (validCertExtensions.every((e) => !file.path.endsWith(e))) {
      _fileError = FileError.invalidFormat;
    } else if (!_validateCertificate(file)) {
      _fileError = FileError.invalidFormat;
    } else {
      _fileError = FileError.none;
    }

    return _fileError == FileError.none;
  }

  bool _validateCertificate(File file) {
    final content = file.readAsBytesSync();

    try {
      X509.fromDer(content);
      return true;
      // Various Exception or Errors can occur here when attempting a parse
      // ignore: avoid_catches_without_on_clauses
    } catch (_) {
      try {
        X509.fromPem(utf8.decode(content));
        return true;
      } on Exception catch (_) {
        return false;
      }
    }
  }

  set sslKeyPath(String value) {
    if (_validatePath(value)) {
      _sslKeyPath = value;
    }
  }

  @override
  bool get isComplete =>
      fqdnError == FqdnError.none &&
      fqdn.isNotEmpty &&
      fileError == FileError.none &&
      !hasAccountNameError;

  @override
  String? config() {
    if (!isComplete) return null;
    assert(_fqdn != null, 'FQDN should not be null at this point');
    // Silly reference to please the analyzer about null checks.
    var fqdn = _fqdn;
    if (fqdn == null) return null;

    final sslKeyLine = sslKeyPath.isEmpty ? '' : 'ssl_public_key = $sslKeyPath';
    final registrationKeyLine =
        registrationKey.isEmpty ? '' : 'registration_key = $registrationKey';

    if (!fqdn.hasPort) {
      // Default documented Landscape hostagent service port is 6554.
      fqdn = fqdn.replace(port: 6554);
    }

    if (!fqdn.hasScheme) {
      // Default documented Landscape hostagent service scheme is https.
      fqdn = fqdn.replace(scheme: 'https');
    }

    // Port should be defined by the scheme, the FQDN one is dedicated to the host agent.
    final clientUrl = Uri(
      scheme: fqdn.scheme,
      host: fqdn.host,
      path: '/message-system',
    );

    // The ping URL is always HTTP.
    final pingUrl = Uri(
      scheme: 'http',
      host: fqdn.host,
      path: '/ping',
    );

    return '''
[host]
url = ${fqdn.host}:${fqdn.port}
[client]
account_name = $_accountName
url = $clientUrl
ping_url = $pingUrl
log_level = info
$sslKeyLine
$registrationKeyLine
'''
        .trimRight();
  }
}

/// The custom configuration form data: the only field available is the path to the configuration file.
class LandscapeCustomConfig extends LandscapeConfig {
  String _configPath = '';
  String get configPath => _configPath;
  FileError _fileError = FileError.none;
  FileError get fileError => _fileError;

  // The provided path must exist and be a non-empty file with bounded size.
  bool _validatePath(String path) {
    if (path.isEmpty) {
      _fileError = FileError.emptyPath;
      return false;
    }

    final fileStat = File(path).statSync();
    if (fileStat.type == FileSystemEntityType.notFound) {
      _fileError = FileError.notFound;
    } else if (fileStat.type == FileSystemEntityType.directory) {
      _fileError = FileError.dir;
    } else if (fileStat.size == 0) {
      _fileError = FileError.emptyFile;
    } else if (fileStat.size >= 1024 * 1024) {
      _fileError = FileError.tooLarge;
    } else {
      _fileError = FileError.none;
    }

    return _fileError == FileError.none;
  }

  set configPath(String value) {
    if (_configPath == value) {
      return;
    }
    if (_validatePath(value)) {
      _configPath = value;
    }
  }

  @override
  bool get isComplete => fileError == FileError.none && configPath.isNotEmpty;

  @override
  String? config() {
    if (!isComplete) return null;
    final file = File(configPath);
    return file.readAsStringSync();
  }
}
