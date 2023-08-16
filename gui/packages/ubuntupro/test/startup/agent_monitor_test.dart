import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:path/path.dart' as p;

import 'package:ubuntupro/constants.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/environment.dart';
import 'package:ubuntupro/pages/startup/agent_monitor.dart';

import 'agent_monitor_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  const kTimeout = Duration(seconds: 3);
  const kInterval = Duration(milliseconds: 100);

  Directory? appDir;
  setUpAll(() async {
    // Overrides the LOCALAPPDATA value to point to a temporary directory and
    // creates the agent directory inside it, where we should find the addr file.
    // Returns the mocked LOCALAPPDATA value for later deletion.
    final tmp = await Directory.current.createTemp();
    final _ = Environment(
      overrides: {'LOCALAPPDATA': tmp.path},
    );

    appDir = await Directory(p.join(tmp.path, kAppName)).create();
  });
  tearDownAll(() async {
    await appDir?.parent.delete(recursive: true);
  });

  test('agent cannot start', () async {
    final mockClient = MockAgentApiClient();

    final monitor = AgentStartupMonitor(
      /// A launch request will always fail.
      agentLauncher: () async => false,
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    expect(
      monitor.start(interval: kInterval, timeout: kTimeout),
      emitsInOrder([
        AgentState.querying,
        AgentState.cannotStart,
        emitsDone,
      ]),
    );

    verifyNever(mockClient.ping());
  });

  test('ping non responsive', () async {
    writeDummyAddrFile(appDir!);

    // Fakes a ping failure.
    final mockClient = MockAgentApiClient();
    when(mockClient.ping()).thenAnswer((_) async => false);

    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async => true,
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    expect(
      monitor.start(interval: kInterval, timeout: kTimeout),
      emitsInOrder([
        AgentState.querying,
        AgentState.pingNonResponsive,
        AgentState.unreachable,
        emitsDone,
      ]),
    );
  });

  test('format error', () async {
    writeDummyAddrFile(appDir!, line: 'Hello, 45567');

    final mockClient = MockAgentApiClient();
    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async => true,
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    expect(
      monitor.start(interval: kInterval, timeout: kTimeout),
      emitsInOrder([
        AgentState.querying,
        AgentState.invalid,
        AgentState.unreachable,
        emitsDone,
      ]),
    );
    verifyNever(mockClient.ping());
  });

  test('access denied', () async {
    final mockClient = MockAgentApiClient();
    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async => true,
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    await IOOverrides.runZoned(
      () async {
        expect(
          monitor.start(interval: kInterval, timeout: kTimeout),
          emitsInOrder([
            AgentState.querying,
            AgentState.unknownEnv,
            emitsDone,
          ]),
        );
        verifyNever(mockClient.ping());
      },
      createFile: (_) => throw const FileSystemException('access denied'),
    );
  });

  test('already running with mocks', () async {
    writeDummyAddrFile(appDir!);

    final mockClient = MockAgentApiClient();
    // Fakes a successful ping.
    when(mockClient.ping()).thenAnswer((_) async => true);
    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async => true,
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    await expectLater(
      monitor.start(interval: kInterval, timeout: kTimeout),
      emitsInOrder([
        AgentState.querying,
        AgentState.ok,
        emitsDone,
      ]),
    );
    verify(mockClient.ping()).called(1);
  });

  test('start agent with mocks', () async {
    final mockClient = MockAgentApiClient();
    // Fakes a successful ping.
    when(mockClient.ping()).thenAnswer((_) async => true);
    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async {
        writeDummyAddrFile(appDir!);
        return true;
      },
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) {},
    );

    await expectLater(
      monitor.start(interval: kInterval),
      emitsInOrder([
        AgentState.querying,
        AgentState.starting,
        AgentState.ok,
        emitsDone,
      ]),
    );
    verify(mockClient.ping()).called(1);
  });

  test('await async onClient callback', () async {
    final completeMe = Completer<void>();
    final mockClient = MockAgentApiClient();
    // Fakes a successful ping.
    when(mockClient.ping()).thenAnswer((_) async => true);
    final monitor = AgentStartupMonitor(
      /// A launch request will always succeed.
      agentLauncher: () async {
        writeDummyAddrFile(appDir!);
        return true;
      },
      clientFactory: (port) => mockClient,
      appName: kAppName,
      addrFileName: kAddrFileName,
      onClient: (_) async {
        // This function only completes when the completer is manually set complete.
        await completeMe.future;
      },
    );

    // As broadcast stream to allow more than one expectLater expressions.
    final stream = monitor.start(interval: kInterval).asBroadcastStream();
    await expectLater(
      stream,
      emitsInOrder([
        AgentState.querying,
        AgentState.starting,
        // Adding more states to this list will block and cause the test to fail
        // because the async onClient callback will never complete.
      ]),
    );

    // Now the async onClient is allowed to return and the stream should output the final states.
    completeMe.complete();
    await expectLater(
      stream,
      emitsInOrder([
        AgentState.ok,
        emitsDone,
      ]),
    );
  });
}

/// Writes a sample `addr` file to the destination containing either a proper
/// contents as if the agent would have written it or [line], if supplied.
void writeDummyAddrFile(Directory appDir, {String? line}) {
  final filePath = p.join(appDir.path, 'addr');
  const port = 56789;
  const goodLine = '[::]:$port';
  final addr = File(filePath);
  addr.writeAsStringSync(line ?? goodLine);
  addTearDown(addr.deleteSync);
}
