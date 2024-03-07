import 'dart:async';

import 'package:flutter/foundation.dart';

import 'agent_api_client.dart';
import 'agent_monitor.dart';

class AgentConnection extends ChangeNotifier {
  bool _isConnected = false;
  bool get isConnected => _isConnected;

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
      _isConnected = state;
      notifyListeners();
    });
    // If we got a stream subscription we have an active connection.
    _isConnected = _connectivitySubscription != null;
    notifyListeners();
  }

  Future<void> restartAgent() async {
    await _connectivitySubscription?.cancel();
    _isConnected = false;
    notifyListeners();
    await monitor.reset();
    _isConnected = await monitor.start().last == AgentState.ok;
    notifyListeners();
    _refreshSubscription(monitor.agentApiClient);
  }

  @override
  void dispose() {
    _connectivitySubscription?.cancel();
    super.dispose();
  }
}
