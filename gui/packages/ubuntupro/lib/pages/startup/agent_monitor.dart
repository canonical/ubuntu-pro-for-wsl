import 'dart:async';
import 'dart:io';

import '../../core/agent_api_client.dart';
import '../../core/agent_api_paths.dart';

enum AgentState {
  /// Querying agent state, not yet known.
  querying,

  /// Agent start request completes successfully.
  starting,

  /// Agent cannot be started.
  cannotStart,

  /// Agent assumed to be running but not responding to PING requests. Some wait might be enough.
  pingNonResponsive,

  /// Agent must be in some kind of transient corrupted state.
  invalid, // such as addr file existing but empty.

  /// Agent assumed to be running but cannot be accessed.
  unreachable, //  Such as invalid addr file contents for too long or not responding to many PING requests.

  /// The system cannot provide us the location where the file is supposed to be, thus we cannot know in which state the agent is.
  unknownEnv,

  /// Agent is up and running.
  ok;

  /// Returns true if no further state changes are expected.
  bool isTerminal() {
    return this == ok ||
        this == cannotStart ||
        this == unknownEnv ||
        this == unreachable;
  }
}

/// A Function that knows how to create an AgentApiClient from a port.
typedef ApiClientFactory = AgentApiClient Function(int port);

/// A Function that knows how to launch the agent and report success.
typedef AgentLauncher = Future<bool> Function();

/// A Callback for when/if the Agent API client becomes available.
typedef AgentApiCallback = FutureOr<void> Function(AgentApiClient);

class AgentStartupMonitor {
  AgentStartupMonitor({
    required String appName,
    required String addrFileName,
    required this.agentLauncher,
    required this.clientFactory,
    required this.onClient,
  }) : _addrFilePath = agentAddrFilePath(appName, addrFileName);

  final String? _addrFilePath;

  /// To launch the agent if it's down.
  final AgentLauncher agentLauncher;

  /// To create a client once the agent is up and running.
  final ApiClientFactory clientFactory;

  /// The callback to invoke once the client is responsive.
  final AgentApiCallback onClient;

  /// Models the background agent as seen by the GUI as a state machine, i.e.:
  /// 1. Agent running state is checked (by looking for the `addr` file).
  /// 2. Agent start is requested by calling [agentLaucher] if not running.
  /// 3. Contents of the `addr` file are scanned periodically (between [interval]).
  /// 4. When a port is available, [clientFactory] is called to create a new
  ///    [AgentApiClient].
  /// 5. When a PING request succeeds, the [onClient] function is called with
  ///    that [AgentApiClient] instance.
  ///
  /// The loop stops if a terminal condition is found or [timeout] expires.
  Stream<AgentState> start({
    Duration interval = const Duration(seconds: 1),
    Duration timeout = const Duration(seconds: 5),
  }) async* {
    if (_addrFilePath == null) {
      // Terminal state, cannot recover nor retry.
      yield AgentState.unknownEnv;
      return;
    }

    yield AgentState.querying;

    yield* delay(
      _checkAgentInLoop(),
      interval,
      // Only emits new state, i.e. if the checkAgentInLoop method returns the
      // same value as before, the stream ignores it and the loop proceeds.
    ).distinct().timeout(
      timeout,
      onTimeout: (sink) {
        // If a timeout happens the unreachable state is emmited and we stop.
        sink.add(AgentState.unreachable);
        sink.close();
      },
    );
  }

  Stream<AgentState> _checkAgentInLoop() async* {
    // This loop seems eager, but Streams are lazy, so it will obey to the
    // caller's request, which can even impose wait intervals between subsequent
    // calls.
    for (var st = AgentState.querying; !st.isTerminal();) {
      final portResult = await readAgentPortFromFile(_addrFilePath!);
      st = await portResult.fold(
        ifLeft: _onAddrError,
        ifRight: _onPort,
      );
      yield st;
    }
  }

  Future<AgentState> _onAddrError(
    AgentAddrFileError error,
  ) async {
    switch (error) {
      case AgentAddrFileError.accessDenied:
        // The system pointed to a location where we cannot read.
        // Terminal state, cannot recover nor retry.
        return AgentState.unknownEnv;
      case AgentAddrFileError.nonexistent:
        if (!await agentLauncher()) {
          // Terminal state, cannot recover nor retry.
          return AgentState.cannotStart;
        }
        return AgentState.starting;
      // maybe a race condition allowed us to read the file before write completed? Retry.
      // ignore: switch_case_completes_normally
      case AgentAddrFileError.isEmpty:
      case AgentAddrFileError.formatError:
        return AgentState.invalid;
    }
  }

  Future<AgentState> _onPort(int port) async {
    final client = clientFactory(port);
    if (await client.ping()) {
      await onClient(client);
      return AgentState.ok;
    }

    return AgentState.pingNonResponsive;
  }

  /// Assumes the agent crashed, i.e. the `addr` file exists but the agent
  /// cannot respond to PING requets.
  /// Thus, we delete the existing `addr` file and retry launching the agent.
  Future<void> reset() async {
    if (_addrFilePath != null) {
      await File(_addrFilePath!).delete();
    }
  }
}

/// Awaits [duration] between [inputStream] events. If [inputStream] is a generator
/// function, it means it will only be invoked after [duration] elapsed.s
Stream<T> delay<T>(Stream<T> inputStream, Duration duration) async* {
  await for (final val in inputStream) {
    yield val;
    await Future.delayed(duration);
  }
}
