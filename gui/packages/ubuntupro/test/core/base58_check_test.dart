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
      expect(base58.checkDecode(' '), equals(B58Error.invalidFormat));
    }
  });
  test('base 58 no errors', () {
    const table = [
      '923EWFTMwpJNkmp',
      '5NXNrWAFJtV2CXu23AfaUGtDA9kzreQ4NYMQc',
      '6upsMkjncyvghsh1Dosg2n5hHj',
      '7Udovn9QXcSM7rTnb6oG4MoFrsvWvcZPm6E4QLrAp',
    ];
    for (final element in table) {
      expect(base58.checkDecode(element), isNull);
    }
  });

  test('simulating real data', () {
    const token = 'C5NXNrWAFJtV2CXu23AfaUGtDA9kzreQ4NYMQc';
    // ignore: avoid_print
    print('\tThis looks like a contract token: "$token"');
    expect(base58.checkDecode(token.substring(1, token.length)), isNull);
  });
}
