import 'dart:async';

import 'package:flutter/foundation.dart';

import 'agent_api_client.dart';
import 'agent_monitor.dart';

class AgentConnection extends ChangeNotifier {
  AgentConnectionState _state = AgentConnectionState.disconnected;
  AgentConnectionState get state => _state;

  StreamSubscription<bool>? _connectivitySubscription;

  final AgentStartupMonitor monitor;

  /// Initializes the AgentConnection with a monitor, requesting notification when the agent API client
  /// becomes available or changes its connection state, if it's already up and running.
  AgentConnection(this.monitor) {
    if (!monitor.addNewClientListener(_refreshSubscription)) {
      _refreshSubscription(monitor.agentApiClient);
    }
  }

  void _refreshSubscription(AgentApiClient? client) {
    _connectivitySubscription = client?.onConnectionChanged
        .map((event) => event == ConnectionEvent.connected)
        .listen((state) {
      _state = state
          ? AgentConnectionState.connected
          : AgentConnectionState.disconnected;
      notifyListeners();
    });
    // If we got a stream subscription we have an active connection.
    _state = _connectivitySubscription != null
        ? AgentConnectionState.connected
        : AgentConnectionState.disconnected;
    notifyListeners();
  }

  Future<void> restartAgent() async {
    await _connectivitySubscription?.cancel();
    _state = AgentConnectionState.connecting;
    notifyListeners();

    final monitorEvent = await monitor.start().last;
    if (monitorEvent != AgentState.ok) {
      _state = AgentConnectionState.disconnected;
      notifyListeners();
      return;
    }
    _state = AgentConnectionState.connected;
    _refreshSubscription(monitor.agentApiClient);
    notifyListeners();
  }

  @override
  void dispose() {
    _connectivitySubscription?.cancel();
    super.dispose();
  }
}

enum AgentConnectionState { connected, connecting, disconnected }
