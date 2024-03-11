import 'package:flutter/material.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:windows_single_instance/windows_single_instance.dart';
import 'package:yaru/widgets.dart';
import 'app.dart';
import 'constants.dart';
import 'core/agent_api_client.dart';
import 'core/agent_monitor.dart';
import 'launch_agent.dart';

Future<void> main() async {
  await YaruWindowTitleBar.ensureInitialized();
  await WindowsSingleInstance.ensureSingleInstance(
    [],
    'UP4W_SINGLE_INSTANCE_GUI',
  );
  final agentMonitor = AgentStartupMonitor(
    addrFileName: kAddrFileName,
    agentLauncher: launch,
    clientFactory: defaultClient,
    onClient: registerServiceInstance<AgentApiClient>,
  );
  runApp(Pro4WSLApp(agentMonitor));
}

AgentApiClient defaultClient(String host, int port) =>
    AgentApiClient(host: host, port: port);

Future<bool> launch() => launchAgent(kAgentRelativePath);
