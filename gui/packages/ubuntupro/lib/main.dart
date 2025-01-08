import 'package:flutter/material.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:window_manager/window_manager.dart';
import 'package:windows_single_instance/windows_single_instance.dart';
import 'package:yaru/widgets.dart';

import 'app.dart';
import 'constants.dart';
import 'core/agent_api_client.dart';
import 'core/agent_monitor.dart';
import 'core/environment.dart';
import 'core/settings.dart';
import 'launch_agent.dart';

Future<void> main() async {
  await YaruWindowTitleBar.ensureInitialized();
  await WindowsSingleInstance.ensureSingleInstance(
    [],
    'UP4W_SINGLE_INSTANCE_GUI',
  );
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();

  final agentMonitor = AgentStartupMonitor(
    addrFileName: kAddrFileName,
    agentLauncher: launch,
    clientFactory: AgentApiClient.new,
    onClient: registerServiceInstance<AgentApiClient>,
  );

  final settings = Environment()['UP4W_INTEGRATION_TESTING'] != null
      ? Settings.withOptions(Options.withAll)
      : Settings(SettingsRepository());

  final windowOptions = const WindowOptions(
    size: Size(kWindowWidth, kWindowHeight),
    minimumSize: Size(kWindowWidth, kWindowHeight),
    center: true,
  );
  await windowManager.waitUntilReadyToShow(windowOptions, () async {
    await windowManager.show();
    await windowManager.focus();
  });

  runApp(Pro4WSLApp(agentMonitor, settings));
}

Future<bool> launch() => launchAgent(kAgentRelativePath);
