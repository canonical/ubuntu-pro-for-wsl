import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:ubuntu_logger/ubuntu_logger.dart';

import '/core/agent_monitor.dart';

final _log = Logger('startup');

enum ViewState { inProgress, ok, retry, crash }

extension ViewStateX on AgentState {
  ViewState toViewState() {
    switch (this) {
      case AgentState.pingNonResponsive:
      case AgentState.invalid:
      case AgentState.querying:
      case AgentState.starting:
        return ViewState.inProgress;
      case AgentState.unreachable:
        return ViewState.retry;
      case AgentState.cannotStart:
      case AgentState.unknownEnv:
        return ViewState.crash;
      case AgentState.ok:
        return ViewState.ok;
    }
  }
}

/// A view-model providing listeners with the up-to-date state of the Windows
/// Background Agent's startup by subscribing the [AgentStartupMonitor] supplied.
class StartupModel extends ChangeNotifier {
  StartupModel(this.monitor);
  final AgentStartupMonitor monitor;

  ViewState _view = ViewState.inProgress;
  // Provides the details of the agent state. Useful for generating user-facing messages.
  AgentState _agentState = AgentState.querying;
  AgentState get details => _agentState;
  ViewState get view => _view;

  StreamSubscription<AgentState>? _subs;
  static const _retryLimit = 5;
  int _retries = 0;

  /// Starts the monitor and subscribes to its events. Returns a future that
  /// completes when the agent monitor startup routine completes.
  Future<void> init() {
    final completer = Completer<void>();
    final stream = monitor.start();
    _subs = stream.listen((state) {
      _log.debug('Received agent state $state');
      _agentState = state;
      _view = state.toViewState();
      notifyListeners();
    }, onDone: completer.complete);
    return completer.future;
  }

  /// Assumes the agent crashed, i.e. the address file exists but the agent cannot respond to PING requets.
  /// Thus, we delete the existing address file and try launching the agent.
  Future<void> resetAgent() async {
    assert(
      _view == ViewState.retry,
      "resetAgent only if it's possible to retry",
    );
    if (_retries >= _retryLimit) {
      _view = ViewState.crash;
      notifyListeners();
      return;
    }
    ++_retries;
    await monitor.reset();
    await _subs?.cancel();
    return init();
  }

  @override
  void dispose() {
    _subs?.cancel();
    super.dispose();
  }
}
