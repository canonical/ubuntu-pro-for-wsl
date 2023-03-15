@TestOn('windows')
import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';

void main() {
  test('dir should not contain "Roaming"', () {
    const appName = 'AwesomeApp';

    final dir = agentAddrFilePath(appName, 'addr');

    expect(dir.contains('Roaming'), isFalse);
    expect(dir.contains('Local'), isTrue);
    expect(dir.contains(appName), isTrue);
  });

  test('read port from line', () {
    const port = 56768;
    const line = '[::]:$port';

    // Exercises the parsing algorithm.
    final res = readAgentPortFromLine(line);

    expect(res, port);
  });

  test('read port from addr file', () async {
    const filePath = './addr';
    const port = 56768;
    const line = '[::]:$port';
    final addr = File(filePath);
    addr.writeAsStringSync(line);

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFromFile(filePath);

    expect(res.orNull(), port);
  });
}
