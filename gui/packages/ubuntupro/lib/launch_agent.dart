import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:path/path.dart' as p;

/// Starts the Windows background agent from its well-known location relative
/// to the root of the deployed application package [agentRelativePath].
//
// The packaging imposes that the agent's path relative to the gui is:
// ../agent/ubuntu-pro-agent.exe
// We don't provide different behavior in debug vs release to ensure maximum
// coverage of the code run in production during test and debugging.
// TODO: Compile and place the agent at the right place while in development.
// The above can be achieved by setting compile time constants with `--dart-define`
// and adding the required code to compile and move the binary to the right place.
// We should strive to still run the same code we run in production.
Future<bool> launchAgent(String agentRelativePath) async {
  final thisDir = File(Platform.resolvedExecutable).parent;
  final agentPath = p.join(thisDir.parent.path, agentRelativePath);
  try {
    await Process.start(
      agentPath,
      [],
    );
    return true;
  } on ProcessException catch (err) {
    // TODO: Proper logging.
    //ignore: avoid_print
    debugPrint(err.message);
    return false;
  }
}
