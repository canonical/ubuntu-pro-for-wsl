import 'dart:io';

import 'package:dart_either/dart_either.dart';
import 'package:path/path.dart' as p;

import 'environment.dart';

/// Provides the full path of the "[filename]" file
/// under the well known directory where the Windows Agent stores its local data.
/// Returns null if that directory location cannot be determined from the environment.
String? agentAddrFilePath(String filename) {
  final homeDir = Environment.instance['USERPROFILE'];
  if (homeDir != null) {
    return p.join(homeDir, filename);
  }

  return null;
}

enum AgentAddrFileError { nonexistent, isEmpty, formatError, accessDenied }

/// Reads the agent port from the addr file located at the full path [filepath].
Future<Either<AgentAddrFileError, int>> readAgentPortFromFile(
  String filepath,
) async {
  try {
    final addr = File(filepath);
    // This returns false without crashing even if the [filepath] is invalid.
    if (!await addr.exists()) {
      // error: file doesn't exist.
      return const Left(AgentAddrFileError.nonexistent);
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
  } on FileSystemException catch (_) {
    // error: permission denied
    return const Left(AgentAddrFileError.accessDenied);
  }
}

/// Parses [line] assuming it's from Windows Agent addr file. Returns null on error.
int? readAgentPortFromLine(String line) => int.tryParse(line.split(':').last);
