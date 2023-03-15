@TestOn('windows')
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:path/path.dart' as p;
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/agent_api_paths.dart';

Future<Process> startAgent() async {
  final mainGo = p.join(
    Directory.current.parent.parent.parent.path,
    'windows-agent/cmd/ubuntu-pro-agent/main.go',
  );
  final agent = Process.start(
    'go',
    ['run', mainGo, '-vvv'],
    mode: ProcessStartMode.inheritStdio,
  );

  final file = agentAddrFilePath('Ubuntu Pro', 'addr');

  await File(file)
      .parent
      .watch(events: FileSystemEvent.modify, recursive: true)
      .take(1)
      // ignore: avoid_print
      .forEach(print);

  return agent;
}

void main() {
  test('ping fails', timeout: const Timeout(Duration(seconds: 5)), () async {
    final client = AgentApiClient(host: '127.0.0.1', port: 81);
    // There should be no service running at this port.
    expect(await client.ping(), isFalse);
  });

  final skip = Platform.environment['GOPATH'] == null
      ? 'Those tests require Go to start the agent'
      : false;

  // The following group is conditionally skipped based on the absence of the
  // GOPATH environment variable.
  group('with a real agent', skip: skip, () {
    Process? agent;
    AgentApiClient? client;
    setUp(() async {
      agent = await startAgent();
      final port = await readAgentPortFromFile(
        agentAddrFilePath('Ubuntu Pro', 'addr'),
      );
      client = AgentApiClient(host: '127.0.0.1', port: port);
    });

    tearDown(() => agent!.kill());
    test('ping succeeds', () async {
      expect(await client!.ping(), isTrue);
    });
    test('pro attach', () async {
      // This currently errors out because of implementation details on the agent.
      // soon this will require a better implementation.
      try {
        await client!.proAttach('C123');
      } on GrpcError catch (err) {
        expect(err.code, StatusCode.unknown);
      }
    });
  });
}
