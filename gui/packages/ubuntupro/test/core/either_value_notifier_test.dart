import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/either_value_notifier.dart';

class NonZeroIntNotifier extends EitherValueNotifier<String, int> {
  NonZeroIntNotifier() : super.ok(42);

  void update(int candidate) {
    if (candidate == 0) {
      value = const Left('Value cannot be zero');
      return;
    }
    value = Right(candidate);
  }
}

void main() {
  final notifier = NonZeroIntNotifier();
  test('fails to the left type', () async {
    notifier.update(0);

    expect(notifier.errorOrNull, isInstanceOf<String>());
    expect(notifier.errorOrNull, contains('zero'));
  });
  test('notify listeners automatically', () async {
    var notified = 0;
    notifier.addListener(() {
      notified++;
    });

    notifier.update(51);
    expect(notifier.errorOrNull, isNull);
    expect(notifier.valueOrNull, equals(51));
    expect(notified, equals(1));

    // Even on error
    notifier.update(0);
    expect(notifier.errorOrNull, isNotNull);
    expect(notifier.valueOrNull, isNull);
    expect(notified, equals(2));

    notifier.update(10);
    expect(notifier.errorOrNull, isNull);
    expect(notifier.valueOrNull, equals(10));
    expect(notified, equals(3));
  });
  test('should not notify on the same value', () async {
    var notified = 0;
    notifier.addListener(() {
      notified++;
    });

    notifier.update(51);
    expect(notifier.errorOrNull, isNull);
    expect(notifier.valueOrNull, equals(51));
    expect(notified, equals(1));

    notifier.update(51);
    expect(notifier.errorOrNull, isNull);
    expect(notifier.valueOrNull, equals(51));
    expect(notified, equals(1));

    // Notifies on error
    notifier.update(0);
    expect(notifier.errorOrNull, isNotNull);
    expect(notifier.valueOrNull, isNull);
    expect(notified, equals(2));

    // But not again on the same error
    notifier.update(0);
    expect(notifier.errorOrNull, isNotNull);
    expect(notifier.valueOrNull, isNull);
    expect(notified, equals(2));
  });
}
