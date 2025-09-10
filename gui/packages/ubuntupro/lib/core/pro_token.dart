import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';

import 'base58_check.dart';

/// The possible errors when parsing a candidate Pro token string.
enum TokenError { empty, invalid }

final _b58 = Base58();

TokenError? _validate(String? value) {
  if (value == null || value.isEmpty) {
    return TokenError.empty;
  }

  if (value.length < ProToken.minLength ||
      value.length > ProToken.maxLength ||
      value[0] != 'C' ||
      _b58.checkDecode(value.substring(1, value.length)) != null) {
    return TokenError.invalid;
  }

  return null;
}

/// Implements an immutable validated object representing a Pro token.
@immutable
class ProToken {
  /// Constructor intentionally private to force validation.
  const ProToken._(this._value);

  /// The token value as a String.
  String get value => _value;
  final String _value;

  // Returns a partially hidden version of the contents, suitable for logging low-sensitive information.
  // Hidden enough to prevent others from reading the value while still allowing the contents author to recognize it.
  // Useful for reading logs with test data. For example: `Obfuscate("Blahkilull")=="Bl******ll`".
  // This is mostly a port of the common.Obfuscate Go function.
  String get obfuscated {
    const endsToReveal = 2;
    final asterisksLength = value.length - 2 * endsToReveal;
    // No need to check if (asterisksLength < 1) because minLength makes it impossible.

    return value.substring(0, endsToReveal) +
        '*' * asterisksLength +
        value.substring(asterisksLength + endsToReveal);
  }

  /// Either returns a [TokenError] or a [ProToken] instance upon validating
  /// the candidate String [from].
  static Either<TokenError, ProToken> create(String from) {
    final error = _validate(from);
    if (error == null) {
      return Right(ProToken._(from));
    }
    return Left(error);
  }

  @override
  int get hashCode => _value.hashCode;

  /// Token string minimum length.
  static const minLength = 24;

  /// Token string maximum length.
  static const maxLength = 30;

  @override
  bool operator ==(Object other) {
    if (other is ProToken) {
      return _value == other._value;
    }
    return false;
  }
}
