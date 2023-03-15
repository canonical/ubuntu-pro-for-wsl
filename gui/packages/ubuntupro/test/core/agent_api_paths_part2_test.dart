@TestOn('windows')

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';
import 'package:ubuntupro/core/environment.dart';

void main() {
  // Tests had to be split in different files because the Environment is a
  // singleton (a static lasting as long as the application). We need to apply
  // the overrides early in main and they will last until main exits.
  // The singleton is not an issue here because it's only meant for testing.
  // In production it will always be initialized with the empty overrides and
  // that will cost only a branch on if-null. Maybe the compiler can optimize
  // that away. I'm not sure.
  final _ = Environment(
    overrides: {'LOCALAPPDATA': Platform.environment['APPDATA']!},
  );

  test('misleading environment', () {
    const appName = 'AwesomeApp';

    final dir = agentAddrFilePath(appName, 'addr')!;

    expect(dir.contains('Roaming'), isTrue);
    expect(dir.contains(appName), isTrue);
  });
}
