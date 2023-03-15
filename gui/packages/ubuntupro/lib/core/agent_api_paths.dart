import 'dart:io';

import 'package:dart_either/dart_either.dart';
import 'package:path/path.dart' as p;

import 'environment.dart';

/// Provides the full path of the "[appDir]/[filename]" file
/// under the well known directory where the Windows Agent stores its local data.
/// Returns null if that directory location cannot be determined from the environment.
String? agentAddrFilePath(String appDir, String filename) {
// The well-known package path_provider doesn't return the LOCALAPPDATA directory
// but the APPDATA, which is usually under %USERPROFILE%/AppData/Roaming instead of
// %USERPROFILE%/AppData/Local, which is where the agent is storing the support data.
  final localAppDir = Environment.instance['LOCALAPPDATA'];
  if (localAppDir != null) {
    return p.join(localAppDir, appDir, filename);
  }

  final userProfile = Environment.instance['USERPROFILE'];
  if (userProfile != null) {
    return p.join(userProfile, 'AppData', 'Local', appDir, filename);
  }

  return null;
}

enum AgentAddrFileError { inexistent, isEmpty, formatError }

/// Reads the agent port from the addr file located at the full path [filepath].
Future<Either<AgentAddrFileError, int>> readAgentPortFromFile(
  String filepath,
) async {
  final addr = File(filepath);
  // This returns false without crashing even if the [filepath] was invalid.
  if (!await addr.exists()) {
    // error: file doesn't exist.
    return const Left(AgentAddrFileError.inexistent);
  }
  final lines = await addr.readAsLines();
  if (lines.isEmpty) {
    // error: file is empty
    return const Left(AgentAddrFileError.isEmpty);
  }
  final port = readAgentPortFromLine(lines[0]);
  if (port == null) {
    // error: format error
    return const Left(AgentAddrFileError.formatError);
  }
  return Right(port);
}

/// Parses [line] assuming it's from Windows Agent addr file. Returns null on error.
int? readAgentPortFromLine(String line) => int.tryParse(line.split(':').last);
