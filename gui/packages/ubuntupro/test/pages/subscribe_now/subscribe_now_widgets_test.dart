import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_widgets.dart';
import 'token_samples.dart' as tks;

void main() {
  group('pro token value', () {
    test('errors', () async {
      final value = ProTokenValue();

      value.update('');

      expect(value.errorOrNull, TokenError.empty);

      value.update(tks.tooShort);

      expect(value.errorOrNull, TokenError.tooShort);

      value.update(tks.tooLong);

      expect(value.errorOrNull, TokenError.tooLong);

      value.update(tks.invalidPrefix);

      expect(value.errorOrNull, TokenError.invalidPrefix);

      value.update(tks.invalidEncoding);

      expect(value.errorOrNull, TokenError.invalidEncoding);
    });
    test('accessors on success', () {
      final value = ProTokenValue();
      final tokenInstance = ProToken.create(tks.good).orNull();

      value.update(tks.good);

      expect(value.hasError, isFalse);
      expect(value.errorOrNull, isNull);
      expect(value.token, tks.good);
      expect(value.valueOrNull!.value, tks.good);
      expect(value.valueOrNull, tokenInstance);
      expect(value.value, equals(ProToken.create(tks.good)));
    });

    test('notify listeners', () {
      final value = ProTokenValue();
      var notified = false;
      value.addListener(() {
        notified = true;
      });

      value.update(tks.good);

      expect(notified, isTrue);
    });
  });
}
