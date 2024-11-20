import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';
import '../utils/token_samples.dart' as tks;

void main() {
  test('token errors', () async {
    final proToken = ProToken.create('');
    expect(proToken, const Left(TokenError.empty));

    for (final token in tks.invalidTokens) {
      final proToken = ProToken.create(token);
      expect(proToken, const Left(TokenError.invalid));
    }
  });

  test('simulates a valid token', () async {
    final proToken = ProToken.create(tks.good);
    expect(proToken.isRight, isTrue);
  });
}
