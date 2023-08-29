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

  Map<String, String>? _merged;

  /// Returns a merged view of the environment variables mixed with the overrides. Useful when passing to child processes.
  Map<String, String> get merged {
    if (_merged == null) {
      // Start with nullable values because _overrides accepts null values as a way to remove items from the Environment.
      // ignore: omit_local_variable_types
      final Map<String, String?> map = {
        ...Platform.environment,
        ...?_overrides,
      };

      // We then remove the entries where values are null.
      map.removeWhere((key, value) => value == null);

      // And finish with a map of a different type -- non-nullable String values.
      _merged = map.map((key, value) => MapEntry(key, value!));
    }

    return _merged!;
  }
}
