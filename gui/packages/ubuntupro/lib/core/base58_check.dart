
import 'package:base_x/base_x.dart';
import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

enum B58Error {
  invalidChecksum,
  invalidFormat,
}

class Base58 {
  final BaseXCodec _codec =
      BaseXCodec('123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz');

  Uint8List _checksum(Uint8List input) {
    final h = sha256.convert(input.toList());
    final h2 = sha256.convert(h.bytes);
    return Uint8List.fromList(h2.bytes.sublist(0, 4));
  }

  // Prepends a version byte and appends a four byte checksum like the btcd API does.
  // This is only for testing.
  @visibleForTesting
  String checkEncode(Uint8List input) {
    final out = List<int>.empty(growable: true);
    out.add(0x20);
    out.addAll(input);
    final chksum = _checksum(Uint8List.fromList(out));
    out.addAll(chksum);
    return _codec.encode(Uint8List.fromList(out));
  }

  /// Checks whether the [input] contains a base58 encoded value.
  /// Returns null on success or the [B58Error] error detected.
  B58Error? checkDecode(String input) {
    try {
      final decoded = _codec.decode(input); // this can throw ArgumentError.
      if (decoded.length < 5) {
        return B58Error.invalidFormat;
      }

      final cksum = decoded.sublist(decoded.length - 4, decoded.length);
      final newCksum = _checksum(decoded.sublist(0, decoded.length - 4));
      for (var i = 0; i < 4; i++) {
        if (cksum[i] != newCksum[i]) {
          return B58Error.invalidChecksum;
        }
      }

      return null;
    } on ArgumentError catch (_) {
      return B58Error.invalidFormat;
    }
  }
}
