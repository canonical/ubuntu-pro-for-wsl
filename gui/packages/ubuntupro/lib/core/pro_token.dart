import 'package:dart_either/dart_either.dart';
import 'package:flutter/material.dart';

import 'base58_check.dart';

/// The possible errors when parsing a candidate Pro token string.
enum TokenError { empty, tooShort, tooLong, invalidPrefix, invalidEncoding }

final _b58 = Base58();

TokenError? _validate(String? value) {
  if (value == null || value.isEmpty) {
    return TokenError.empty;
  }
  if (value.length < ProToken.minLength) {
    return TokenError.tooShort;
  }
  if (value.length > ProToken.maxLength) {
    return TokenError.tooLong;
  }
  // For now only Contract tokens are expected.
  if (value[0] != 'C') {
    return TokenError.invalidPrefix;
  }
  if (_b58.checkDecode(value.substring(1, value.length)) != null) {
    return TokenError.invalidEncoding;
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
