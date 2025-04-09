@TestOn('windows')
library;

import 'dart:io';
import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';

void main() {
  tearDownAll(() => File('./.address').deleteSync());

  test('read ipv4 host and port from line', () {
    const host = '127.0.0.1';
    const port = 56768;
    const line = '$host:$port';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, (host, port));
  });

  test('read ipv6 host and port from line', () {
    const host = '[::]';
    const port = 56768;
    const line = '$host:$port';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, (host, port));
  });

  test('read localhost and port from line', () {
    const host = 'localhost';
    const port = 56768;
    const line = '$host:$port';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, (host, port));
  });

  test('line parsing error', () {
    const port = 56768;
    const line = '[::]-$port';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, isNull);
  });

  test('Negative port error', () {
    const line = '[::]:-56768';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, isNull);
  });

  test('Zero port parsing error', () {
    const line = '[::]:0';

    // Exercises the parsing algorithm.
    final res = parseAddress(line);

    expect(res, isNull);
  });

  test('read host and port from addr file', () async {
    const filePath = './.address';
    const host = '[::]';
    const port = 56768;
    const line = '$host:$port';
    final addr = File(filePath);
    addr.writeAsStringSync(line);

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFromFile(filePath);

    expect(res.orNull(), (host, port));
  });

  test('invalid file name', () async {
    const filePath = '\\<>';

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFromFile(filePath);

    expect(res, const Left(AgentAddrFileError.nonexistent));
  });

  test('empty file', () async {
    const filePath = './.address';
    final addr = File(filePath);
    addr.writeAsStringSync('');

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFromFile(filePath);

    expect(res, const Left(AgentAddrFileError.isEmpty));
  });

  test('access denied', () async {
    const filePath = './.address';
    final addr = File(filePath);
    addr.writeAsStringSync('');

    await IOOverrides.runZoned(() async {
      // Exercises the expected usage: reading from a file
      final res = await readAgentPortFromFile(filePath);

      expect(res, const Left(AgentAddrFileError.accessDenied));
    }, createFile: (_) => throw const FileSystemException('access denied'));
  });

  test('bad format', () async {
    const filePath = './.address';
    const port = 56768;
    const line = 'Hello World $port';
    final addr = File(filePath);
    addr.writeAsStringSync(line);

    // Exercises the expected usage: reading from a file
    final res = await readAgentPortFromFile(filePath);

    expect(res, const Left(AgentAddrFileError.formatError));
  });
}
