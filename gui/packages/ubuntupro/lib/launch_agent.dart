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
  final thisDir = File(Platform.resolvedExecutable).parent;
  final agentPath = p.join(thisDir.parent.path, agentRelativePath);
  try {
    await Process.start(
      agentPath,
      [],
      environment: Environment.instance.merged,
    );
    return true;
  } on ProcessException catch (err) {
    // TODO: Proper logging.
    //ignore: avoid_print
    debugPrint(err.message);
    return false;
  }
}
