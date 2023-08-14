@TestOn('windows')

import 'dart:io';
import 'package:dart_either/dart_either.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as p;
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';
import 'package:ubuntupro/core/environment.dart';

// TODO: Move this to an integration test suite when we get one.
Future<Process> startAgent() async {
  final mainGo = p.join(
    Directory.current.parent.parent.parent.path,
    'windows-agent/cmd/ubuntu-pro-agent/main.go',
  );
  final agent = Process.start(
    'go',
    ['run', mainGo, '-vvv'],
    mode: ProcessStartMode.inheritStdio,
    environment: Environment.instance.merged,
  );

  final file = agentAddrFilePath('Ubuntu Pro', 'addr')!;

  final runtimeDir = await File(file).parent.create(recursive: true);
  await runtimeDir
      .watch(events: FileSystemEvent.modify, recursive: true)
      .take(1)
      // ignore: avoid_print
      .forEach(print);

  return agent;
}

void main() {
  test('ping fails', timeout: const Timeout(Duration(seconds: 5)), () async {
    final client = AgentApiClient(host: '127.0.0.1', port: 9);
    // IANA discard protol: There should be no service running at this port.
    expect(await client.ping(), isFalse);
  });

  final skip = Platform.environment['GOPATH'] == null
      ? 'These tests require Go to start the agent'
      : false;

  // The following group is conditionally skipped based on the absence of the
  // GOPATH environment variable.
  group('with a real agent', skip: skip, () {
    Directory? tmp;

    Process? agent;
    AgentApiClient? client;
    setUpAll(() async {
      tmp = await Directory.current.createTemp('test-');
      Environment(
        overrides: {'LOCALAPPDATA': tmp!.path},
      );

      agent = await startAgent();
      final port = await readAgentPortFromFile(
        agentAddrFilePath('Ubuntu Pro', 'addr')!,
      );
      // either works or crashes
      client = AgentApiClient(host: '127.0.0.1', port: port.getOrThrow());
    });

    tearDownAll(() async {
      // kill all agent processes.
      agent?.kill();
      if (Platform.isWindows) {
        await Process.run('taskkill.exe', ['/f', '/im', 'main.exe']);
      } else {
        await Process.run(
          'killall',
          [p.basenameWithoutExtension('main')],
        );
      }
      // Finally deletes the directory.
      await tmp?.delete(recursive: true);
    });
    test('ping succeeds', () async {
      expect(await client!.ping(), isTrue);
    });

    test('no subscription info', () async {
      final info = await client!.subscriptionInfo();
      expect(info.productId, isEmpty);
      expect(info.immutable, isFalse);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
    });
    test('pro attach', () async {
      // expect no throw.
      await client!.applyProToken('C123');
    });

    test('user subscription', () async {
      final info = await client!.subscriptionInfo();
      expect(info.productId, isEmpty);
      expect(info.immutable, isFalse);
      expect(info.whichSubscriptionType(), SubscriptionType.user);
    });

    test('pro detach', () async {
      // expect no throw.
      await client!.applyProToken('');
    });

    test('no subscription again', () async {
      final info = await client!.subscriptionInfo();
      expect(info.productId, isEmpty);
      expect(info.immutable, isFalse);
      expect(info.whichSubscriptionType(), SubscriptionType.none);
    });
  });
}
