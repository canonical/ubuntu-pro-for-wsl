import 'dart:io';

import 'package:dart_either/dart_either.dart';
import 'package:path/path.dart' as p;

import '/constants.dart';
import 'environment.dart';

/// Provides the absolute path of the "[filename]" file
/// under the well known directory where the Windows Agent stores its local data.
/// Returns null if that directory location cannot be determined from the environment.
String? absPathUnderAgentPublicDir(String filename) {
  final homeDir = Environment.instance['USERPROFILE'];
  if (homeDir != null) {
    return p.join(homeDir, kAgentPublicDir, filename);
  }

  return null;
}

/// Provides the absolute path of [filename] under the agent's private data directory.
String? absPathUnderAgentPrivateDir(String filename) {
  final localAppData = Environment.instance['LOCALAPPDATA'] ??
      Environment.instance['LocalAppData'];
  if (localAppData != null) {
    return p.join(localAppData, kAgentPrivateDir, filename);
  }

  return null;
}

enum AgentAddrFileError { nonexistent, isEmpty, formatError, accessDenied }

/// Reads the agent host and port from the addr file located at the full path [filepath].
Future<Either<AgentAddrFileError, (String, int)>> readAgentPortFromFile(
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

    final address = parseAddress(lines[0]);
    if (address == null) {
      // New agents publish the WSL Unix socket path in this file. The monitor
      // only uses the file as a readiness signal, so port zero is a sentinel.
      if (lines[0].endsWith(kAgentSocketName)) {
        return Right((lines[0], 0));
      }
      return const Left(AgentAddrFileError.formatError);
    }

    return Right(address);
  } on FileSystemException catch (_) {
    // error: permission denied
    return const Left(AgentAddrFileError.accessDenied);
  }
}

/// Parses [line] assuming it's from Windows Agent addr file.
/// Returns a (host, port) tuple on success, or null on error
(String, int)? parseAddress(String line) {
  final parts = line.split(':');
  if (parts.length < 2) {
    return null;
  }

  final host = parts.sublist(0, parts.length - 1).join(':');
  final port = int.tryParse(parts.last);

  if (port == null) {
    return null;
  }
  if (port <= 0) {
    // Port must be strictly positive.
    return null;
  }

  return (host, port);
}
