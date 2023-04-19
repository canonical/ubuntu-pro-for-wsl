import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

import 'agent_monitor.dart';

enum ViewState {
  inProgress,
  ok,
  retry,
  crash,
}

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

  /// Allows representing the [AgentState] enum as a translatable String.
  String localize(AppLocalizations lang) {
    switch (this) {
      case AgentState.starting:
        return lang.agentStateStarting;
      case AgentState.pingNonResponsive:
        return lang.agentStatePingNonResponsive;
      case AgentState.invalid:
        return lang.agentStateInvalid;
      case AgentState.cannotStart:
        return lang.agentStateCannotStart;
      case AgentState.unknownEnv:
        return lang.agentStateUnknownEnv;
      case AgentState.querying:
        return lang.agentStateQuerying;
      case AgentState.unreachable:
        return lang.agentStateUnreachable;
      case AgentState.ok:
        // This state should not need translations.
        return '';
    }
  }
}

/// A view-model providing listeners with the up-to-date state of the Windows
/// Background Agent's startup by subscribing the [AgentStartupMonitor] supplied.
class StartupModel extends ChangeNotifier {
  StartupModel(this.monitor);
  final AgentStartupMonitor monitor;

  ViewState _view = ViewState.inProgress;
  AgentState _agentState = AgentState.querying;
  ViewState get view => _view;
  String message(AppLocalizations localizations) =>
      _agentState.localize(localizations);

  StreamSubscription<AgentState>? _subs;

  /// Starts the monitor and subscribes to its events. Returns a future that
  /// completes when the agent monitor startup routine completes.
  Future<void> init() {
    final completer = Completer<void>();
    final stream = monitor.start();
    _subs = stream.listen(
      (state) {
        _agentState = state;
        _view = state.toViewState();
        notifyListeners();
      },
      onDone: completer.complete,
    );
    return completer.future;
  }

  /// Assumes the agent crashed, i.e. the `addr` file exists but the agent cannot respond to PING requets.
  /// Thus, we delete the existing `addr` file and try launching the agent.
  Future<void> resetAgent() async {
    assert(
      _view == ViewState.retry,
      "resetAgent only if it's possible to retry",
    );
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
