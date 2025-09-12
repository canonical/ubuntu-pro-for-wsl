import 'package:flutter/foundation.dart' show debugPrint;
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/settings.dart';
import 'package:win32/win32.dart';

import 'settings_test.mocks.dart';

@GenerateMocks([SettingsRepository])
void main() {
  group('with options', () {
    test('all', () {
      final settings = Settings.withOptions(Options.withAll);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isTrue);
    });
    test('Landscape', () {
      final settings = Settings.withOptions(Options.withLandscapeConfiguration);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
    test('purchase', () {
      final settings = Settings.withOptions(Options.withStorePurchase);

      expect(settings.isLandscapeConfigurationEnabled, isFalse);
      expect(settings.isStorePurchaseAllowed, isTrue);
    });
    test('none', () {
      final settings = Settings.withOptions(Options.none);

      expect(settings.isLandscapeConfigurationEnabled, isFalse);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
  });

  test('toString', () {
    final settings = [
      Settings.withOptions(Options.withAll),
      Settings.withOptions(Options.withLandscapeConfiguration),
      Settings.withOptions(Options.withStorePurchase),
      Settings.withOptions(Options.none),
    ];

    for (final s0 in settings) {
      for (final s1 in settings) {
        if (s0 == s1) {
          expect(s0.toString(), equals(s1.toString()));
        } else {
          expect(s0.toString(), isNot(equals(s1.toString())));
        }
      }
    }
  });

  group('from repository', () {
    test('all', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenAnswer((_) {});
      when(
        repository.readInt(Settings.kLandscapeConfigVisibility),
      ).thenReturn(null);
      when(repository.readInt(Settings.kAllowStorePurchase)).thenReturn(1);

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isTrue);
    });
    test('Landscape', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenAnswer((_) {});
      when(
        repository.readInt(Settings.kLandscapeConfigVisibility),
      ).thenReturn(null);
      when(repository.readInt(Settings.kAllowStorePurchase)).thenReturn(0);

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
    test('purchase', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenAnswer((_) {});
      when(
        repository.readInt(Settings.kLandscapeConfigVisibility),
      ).thenReturn(0);
      when(repository.readInt(Settings.kAllowStorePurchase)).thenReturn(1);

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isFalse);
      expect(settings.isStorePurchaseAllowed, isTrue);
    });
    test('none', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenAnswer((_) {});
      when(
        repository.readInt(Settings.kLandscapeConfigVisibility),
      ).thenReturn(0);
      when(repository.readInt(Settings.kAllowStorePurchase)).thenReturn(null);

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isFalse);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
    test('unset (defaults)', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenThrow(
          WindowsException(HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND)));

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
    test('defaults with log', () {
      final repository = MockSettingsRepository();
      when(repository.load()).thenThrow(
        WindowsException(HRESULT_FROM_WIN32(ERROR_ACCESS_DENIED)),
      );

      final settings = Settings(repository);

      expect(settings.isLandscapeConfigurationEnabled, isTrue);
      expect(settings.isStorePurchaseAllowed, isFalse);
    });
  });

  group('repository', () {
    test('no crash if not load', () {
      final r = SettingsRepository();
      expect(r.readInt('AKey'), isNull);
      r.close(); // no crash
    });

    test(
      'no crash on load/close',
      () {
        final r = SettingsRepository();
        // We cannot assert many things as the system may have the key.
        // I'd rather avoid touching the real registry unless we really believe necessary.
        try {
          r.load();
        } on WindowsException catch (err) {
          debugPrint(
            'Exception when loading the repository was expected: ${err.message}',
          );
        }
        r.close(); // no crash
      },
      // depends on the real registry.
      testOn: 'windows',
    );
  });
}
