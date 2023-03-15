import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';

void main() {
  test('token errors', () async {
    var token = ProToken.create('');
    expect(token, const Left(TokenError.empty));

    token = ProToken.create('ZmNb8uQn5zv');
    expect(token, const Left(TokenError.tooShort));

    token = ProToken.create('K2RYDcKfupxwXdWhSAxQPCeiULntKm63UXyx5MvEH2');
    expect(token, const Left(TokenError.tooLong));

    token = ProToken.create('K2RYDcKfupxwXdWhSAxQPCeiULntKm');
    expect(token, const Left(TokenError.invalidPrefix));

    token = ProToken.create('CK2RYDcKfupxwXdWhSAxQPCeiULntK');
    expect(token, const Left(TokenError.invalidEncoding));
  });

  test('simulates a valid token', () async {
    final token = ProToken.create('CJd8MMN8wXSWsv7wJT8c8dDK');
    expect(token.isRight, isTrue);
  });
}
