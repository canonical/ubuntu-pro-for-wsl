import 'dart:io';

/// A singleton managing access to environment variables so we can
/// test overriding them. That's because dart:io's Platform class loads
/// the environment variables at startup and doesn't allow overriding them.
class Environment {
  static Environment? _instance;

  /// The single instance.
  static Environment get instance => _instance ??= Environment._();

  /// The custom overrides.
  final Map<String, String?>? _overrides;

  /// A factory constructor optionally accepting a map of environment variables
  /// and their override values. Calling this factory has effect only once
  /// throught the program's lifetime.
  factory Environment({Map<String, String?>? overrides}) {
    return _instance ??= Environment._(overrides: overrides);
  }

  // Constructor is private.
  Environment._({Map<String, String?>? overrides}) : _overrides = overrides {
    _instance = this;
  }

  /// Checks first in the overrides map and the falls back to
  /// Platform.environment map.
  String? operator [](String key) {
    if (_overrides != null && _overrides!.containsKey(key)) {
      return _overrides![key];
    }

    return Platform.environment[key];
  }
}
