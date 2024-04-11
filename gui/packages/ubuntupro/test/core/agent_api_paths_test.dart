@TestOn('windows')
import 'dart:convert';
import 'dart:io';
import 'package:agentapi/agentapi.dart';
import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';

void main() {
  tearDownAll(() => File('./.address').deleteSync());

  test('read host and port from addr file', () async {
    const filePath = './.address';

    final t = AuthTarget(host: '[::]', port: '56768', authToken: 'token');
    final addr = File(filePath);
    addr.writeAsStringSync(jsonEncode(t.toProto3Json()));

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFile(filePath);
    final authTarget = res.getOrThrow();

    expect(authTarget, isNotNull);
    expect(authTarget.host, t.host);
    expect(authTarget.port, t.port);
    expect(authTarget.authToken, t.authToken);
  });

  test('invalid file name', () async {
    const filePath = '\\<>';

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFile(filePath);

    expect(res, const Left(AgentAddrFileError.nonexistent));
  });

  test('empty file', () async {
    const filePath = './.address';
    final addr = File(filePath);
    addr.writeAsStringSync('');

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFile(filePath);

    expect(res, const Left(AgentAddrFileError.isEmpty));
  });

  test('access denied', () async {
    const filePath = './.address';
    final addr = File(filePath);
    addr.writeAsStringSync('');

    await IOOverrides.runZoned(
      () async {
        // Exercises the expected usage: reading from a file
        final res = await readAgentPortFile(filePath);

        expect(res, const Left(AgentAddrFileError.accessDenied));
      },
      createFile: (_) => throw const FileSystemException('access denied'),
    );
  });

  test('bad format', () async {
    const filePath = './.address';
    const port = 56768;
    const line = 'Hello World $port';
    final addr = File(filePath);
    addr.writeAsStringSync(line);

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFile(filePath);

    expect(res, const Left(AgentAddrFileError.formatError));
  });
}
