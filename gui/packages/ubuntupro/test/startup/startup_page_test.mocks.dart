// Mocks generated by Mockito 5.4.0 from annotations
// in ubuntupro/test/startup/startup_page_test.dart.
// Do not manually edit this file.

// ignore_for_file: no_leading_underscores_for_library_prefixes
import 'dart:async' as _i4;

import 'package:mockito/mockito.dart' as _i1;
import 'package:ubuntupro/core/agent_api_client.dart' as _i2;
import 'package:ubuntupro/pages/startup/agent_monitor.dart' as _i3;

// ignore_for_file: type=lint
// ignore_for_file: avoid_redundant_argument_values
// ignore_for_file: avoid_setters_without_getters
// ignore_for_file: comment_references
// ignore_for_file: implementation_imports
// ignore_for_file: invalid_use_of_visible_for_testing_member
// ignore_for_file: prefer_const_constructors
// ignore_for_file: unnecessary_parenthesis
// ignore_for_file: camel_case_types
// ignore_for_file: subtype_of_sealed_class

class _FakeAgentApiClient_0 extends _i1.SmartFake
    implements _i2.AgentApiClient {
  _FakeAgentApiClient_0(
    Object parent,
    Invocation parentInvocation,
  ) : super(
          parent,
          parentInvocation,
        );
}

/// A class which mocks [AgentStartupMonitor].
///
/// See the documentation for Mockito's code generation for more information.
class MockAgentStartupMonitor extends _i1.Mock
    implements _i3.AgentStartupMonitor {
  MockAgentStartupMonitor() {
    _i1.throwOnMissingStub(this);
  }

  @override
  _i3.AgentLauncher get agentLauncher => (super.noSuchMethod(
        Invocation.getter(#agentLauncher),
        returnValue: () => _i4.Future<bool>.value(false),
      ) as _i3.AgentLauncher);
  @override
  _i3.ApiClientFactory get clientFactory => (super.noSuchMethod(
        Invocation.getter(#clientFactory),
        returnValue: (int port) => _FakeAgentApiClient_0(
          this,
          Invocation.getter(#clientFactory),
        ),
      ) as _i3.ApiClientFactory);
  @override
  _i3.AgentApiCallback get onClient => (super.noSuchMethod(
        Invocation.getter(#onClient),
        returnValue: (_i2.AgentApiClient __p0) {},
      ) as _i3.AgentApiCallback);
  @override
  _i4.Stream<_i3.AgentState> start({
    Duration? interval = const Duration(seconds: 1),
    Duration? timeout = const Duration(seconds: 30),
  }) =>
      (super.noSuchMethod(
        Invocation.method(
          #start,
          [],
          {
            #interval: interval,
            #timeout: timeout,
          },
        ),
        returnValue: _i4.Stream<_i3.AgentState>.empty(),
      ) as _i4.Stream<_i3.AgentState>);
  @override
  _i4.Future<void> reset() => (super.noSuchMethod(
        Invocation.method(
          #reset,
          [],
        ),
        returnValue: _i4.Future<void>.value(),
        returnValueForMissingStub: _i4.Future<void>.value(),
      ) as _i4.Future<void>);
}

/// A class which mocks [AgentApiClient].
///
/// See the documentation for Mockito's code generation for more information.
class MockAgentApiClient extends _i1.Mock implements _i2.AgentApiClient {
  MockAgentApiClient() {
    _i1.throwOnMissingStub(this);
  }

  @override
  _i4.Future<void> applyProToken(String? token) => (super.noSuchMethod(
        Invocation.method(
          #applyProToken,
          [token],
        ),
        returnValue: _i4.Future<void>.value(),
        returnValueForMissingStub: _i4.Future<void>.value(),
      ) as _i4.Future<void>);
  @override
  _i4.Future<bool> ping() => (super.noSuchMethod(
        Invocation.method(
          #ping,
          [],
        ),
        returnValue: _i4.Future<bool>.value(false),
      ) as _i4.Future<bool>);
}
