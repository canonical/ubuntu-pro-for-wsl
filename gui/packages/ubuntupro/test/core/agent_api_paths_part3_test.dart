@TestOn('windows')

import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';
import 'package:ubuntupro/core/environment.dart';

void main() {
  // Because this is a singleton (a static lasting as long as the application)
  // we need to apply the overrides early in main and they will last until
  // main exits.
  final _ = Environment(overrides: {'LOCALAPPDATA': null});

  test('fallback maintain invariants', () {
    const appName = 'AwesomeApp';

    final dir = agentAddrFilePath(appName, 'addr')!;

    expect(dir.contains('Roaming'), isFalse);
    expect(dir.contains('Local'), isTrue);
    expect(dir.contains(appName), isTrue);
  });
}
