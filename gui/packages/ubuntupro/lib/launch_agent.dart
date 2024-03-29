import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:path/path.dart' as p;

import 'core/environment.dart';

/// Starts the Windows background agent from its well-known location relative
/// to the root of the deployed application package [agentRelativePath].
//
// The packaging imposes that the agent's path relative to the gui is:
// ../agent/ubuntu-pro-agent.exe
// We don't provide different behavior in debug vs release to ensure maximum
// coverage of the code run in production during test and debugging.
Future<bool> launchAgent(String agentRelativePath) async {
  final agentPath = p.join(msixRootDir().path, agentRelativePath);
  try {
    // Attempts to kill a possibly stuck agent. Failure is desirable in this case.
    await Process.run('taskkill.exe', ['/f', '/im', p.basename(agentPath)]);
    await Process.start(
      agentPath,
      ['-vv'],
      environment: Environment.instance.merged,
      mode: ProcessStartMode.inheritStdio,
    );
    return true;
  } on ProcessException catch (err) {
    // TODO: Proper logging.
    //ignore: avoid_print
    debugPrint(err.message);
    return false;
  }
}

/// Exposes what is expected to be the MSIX root directory relative to this binary's path.
@visibleForTesting
Directory msixRootDir() => File(Platform.resolvedExecutable).parent.parent;
