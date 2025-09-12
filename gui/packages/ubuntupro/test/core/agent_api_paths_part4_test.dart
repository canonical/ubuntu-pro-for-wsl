@TestOn('windows')
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';
import 'package:ubuntupro/core/environment.dart';

void main() {
  // Because this is a singleton (a static lasting as long as the application)
  // we need to apply the overrides early in main and they will last until
  // main exits.
  final _ = Environment(overrides: {'LOCALAPPDATA': null, 'USERPROFILE': null});

  test('complete failure due environment', () {
    final dir = absPathUnderAgentPublicDir('.ubuntupro/.address');

    expect(dir, isNull);
  });
}
