import 'package:flutter/foundation.dart';
import 'package:win32/win32.dart';
import 'package:win32_registry/win32_registry.dart';

/// Manages the settings for the user interface.
class Settings {
  /// Creates a new instance of [Settings] initialized with options read from [repository],
  /// which is loaded, read from and closed.
  Settings(SettingsRepository repository) {
    if (!repository.load()) return;

    // Enable store purchase if the registry value is 1.
    final purchase =
        repository.readInt(kAllowStorePurchase) == 1
            ? Options.withStorePurchase
            : Options.none;

    // Hide Landscape UI if the registry value is 0.
    final landscape =
        repository.readInt(kLandscapeConfigVisibility) == 0
            ? Options.none
            : Options.withLandscapeConfiguration;

    repository.close();

    _options = purchase | landscape;
  }

  /// Creates a new instance of [Settings] with the specified [options], thus no need to read from the repository.
  /// Useful for integration testing.
  Settings.withOptions(this._options);

  /// By default Landscape is enabled and Store purchase is disabled.
  Options _options = Options.withLandscapeConfiguration;

  bool get isLandscapeConfigurationEnabled =>
      _options & Options.withLandscapeConfiguration;
  bool get isStorePurchaseAllowed => _options & Options.withStorePurchase;

  // constants for the key names only exposed for testing.
  @visibleForTesting
  static const kAllowStorePurchase = 'AllowStorePurchase';
  @visibleForTesting
  static const kLandscapeConfigVisibility = 'LandscapeConfigVisibility';
}

/// Settings options modelled as an enum with bitwise operations, i.e. flags.
enum Options {
  none(0x00),
  withLandscapeConfiguration(0x01),
  withStorePurchase(0x02),
  // all optionss above or'ed.
  withAll(0x03);

  final int options;
  const Options(this.options);
  factory Options._fromInt(int options) =>
      Options.values.firstWhere((e) => e.options == options);

  bool operator &(Options other) => options & other.options != 0;
  Options operator |(Options other) =>
      Options._fromInt(options | other.options);
}

// "Abstracts" reading the settings storage (a.k.a the Windows registry).
class SettingsRepository {
  RegistryKey? _key;

  void close() => _key?.close();
  int? readInt(String name) {
    if (_key == null) return null;
    return _key!.getValueAsInt(name);
  }

  bool load() {
    try {
      _key = Registry.openPath(RegistryHive.currentUser, path: _keyPath);
      return true;
    } on WindowsException {
      // missing key is not an error since we expect them to be set in very few cases.
      // TODO: Log error cases other than ERROR_FILE_NOT_FOUND.
      return false;
    }
  }
}

// The registry key we want to read from.
const _keyPath = r'SOFTWARE\Canonical\UbuntuPro\';
