import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/enter_token/enter_token_model.dart';

void main() {
  test('Model errors', () async {
    final model = EnterProTokenModel();

    model.update('');

    expect(model.errorOrNull, TokenError.empty);

    model.update('ZmNb8uQn5zv');

    expect(model.errorOrNull, TokenError.tooShort);

    model.update('K2RYDcKfupxwXdWhSAxQPCeiULntKm63UXyx5MvEH2');

    expect(model.errorOrNull, TokenError.tooLong);

    model.update('K2RYDcKfupxwXdWhSAxQPCeiULntKm');

    expect(model.errorOrNull, TokenError.invalidPrefix);

    model.update('CK2RYDcKfupxwXdWhSAxQPCeiULntK');

    expect(model.errorOrNull, TokenError.invalidEncoding);
  });
  test('accessors on success', () {
    final model = EnterProTokenModel();
    const token = 'CJd8MMN8wXSWsv7wJT8c8dDK';
    final tokenInstance = ProToken.create(token).orNull();

    model.update(token);

    expect(model.hasError, isFalse);
    expect(model.errorOrNull, isNull);
    expect(model.token, token);
    expect(model.valueOrNull!.value, token);
    expect(model.valueOrNull, tokenInstance);
    expect(model.value, equals(ProToken.create(token)));
  });
  test('notify listeners', () {
    final model = EnterProTokenModel();
    var notified = false;
    model.addListener(() {
      notified = true;
    });
    const token = 'CJd8MMN8wXSWsv7wJT8c8dDK';

    model.update(token);

    expect(notified, isTrue);
  });
}
