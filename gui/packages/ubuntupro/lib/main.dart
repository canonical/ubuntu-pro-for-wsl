import 'dart:io';

import 'package:flutter/material.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:yaru_widgets/yaru_widgets.dart';

import 'app.dart';
import 'constants.dart';
import 'core/agent_api_client.dart';
import 'core/agent_api_paths.dart';

Future<void> main() async {
  final addrFile = agentAddrFilePath(kAppName, kAddrFileName);
  // TODO: Real error handling with related UI state being shown.
  // To be done with the logic to start the agent from the GUI.
  if (addrFile == null) {
    exit(1);
  }
  final portResult = await readAgentPortFromFile(addrFile);
  await portResult.fold(
    ifLeft: (_) => exit(2),
    ifRight: (port) async {
      final client = AgentApiClient(host: kDefaultHost, port: port);
      registerServiceInstance<AgentApiClient>(client);
      await YaruWindowTitleBar.ensureInitialized();
      runApp(const Pro4WindowsApp());
    },
  );
}
