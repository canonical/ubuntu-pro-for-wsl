import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/base58_check.dart';

void main() {
  final base58 = Base58();
  test('base 58 errors', () {
    expect(base58.checkDecode('3MNQE1Y'), equals(B58Error.invalidChecksum));
    var testString = '';
    for (var len = 0; len < 5; len++) {
      testString += 'x';
      expect(base58.checkDecode(testString), equals(B58Error.invalidFormat));
    }
  });
  test('base 58 NO errors', () {
    const table = [
      'Ubuntu',
      'What a wonderful world',
      'Immigrant song',
      'Rock you like a hurricane'
    ];
    for (final element in table) {
      final raw = Uint8List.fromList(utf8.encode(element));
      final encoded = base58.checkEncode(raw);
      expect(base58.checkDecode(encoded), isNull);
    }
  });

  test('simulating real data', () {
    final raw = Uint8List.fromList(utf8.encode('Hello World'));
    final encoded = base58.checkEncode(raw);
    final token = 'C$encoded';
    // ignore: avoid_print
    print('\tThis looks like a contract token: "$token"');
    expect(base58.checkDecode(token.substring(1, token.length)), isNull);
  });
}
