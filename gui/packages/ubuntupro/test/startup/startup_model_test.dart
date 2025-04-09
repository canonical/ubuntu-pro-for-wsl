import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/agent_monitor.dart';
import 'package:ubuntupro/pages/startup/startup_model.dart';

import 'startup_model_test.mocks.dart';

@GenerateMocks([AgentStartupMonitor])
void main() {
  group('crash cannot retry', () {
    test('bad env', () async {
      final monitor = MockAgentStartupMonitor();
      when(
        monitor.start(),
      ).thenAnswer((_) => Stream.fromIterable([AgentState.unknownEnv]));

      final model = StartupModel(monitor);
      addTearDown(model.dispose);

      await model.init();

      expect(model.view, ViewState.crash);
      expect(model.resetAgent(), throwsAssertionError);
    });
    test('start failure', () async {
      final monitor = MockAgentStartupMonitor();
      when(monitor.start()).thenAnswer(
        (_) =>
            Stream.fromIterable([AgentState.querying, AgentState.cannotStart]),
      );

      final model = StartupModel(monitor);
      addTearDown(model.dispose);

      await model.init();

      expect(model.view, ViewState.crash);
      expect(model.resetAgent(), throwsAssertionError);
    });
  });

  test('reset', () async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable([
        AgentState.querying,
        AgentState.starting,
        AgentState.invalid,
        AgentState.unreachable,
      ]),
    );

    final model = StartupModel(monitor);
    addTearDown(model.dispose);

    await model.init();

    expect(model.view, ViewState.retry);

    when(monitor.reset()).thenAnswer((realInvocation) async {});
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable([
        AgentState.querying,
        AgentState.starting,
        AgentState.ok,
      ]),
    );

    await model.resetAgent();

    expect(model.view, ViewState.ok);
  });

  test('notify listeners', () async {
    final monitor = MockAgentStartupMonitor();
    when(monitor.start()).thenAnswer(
      (_) => Stream.fromIterable([
        AgentState.querying,
        AgentState.starting,
        AgentState.ok,
      ]),
    );

    final model = StartupModel(monitor);
    addTearDown(model.dispose);

    var notified = false;
    model.addListener(() {
      notified = true;
    });

    await model.init();

    expect(model.view, ViewState.ok);
    expect(notified, isTrue);
  });
}
