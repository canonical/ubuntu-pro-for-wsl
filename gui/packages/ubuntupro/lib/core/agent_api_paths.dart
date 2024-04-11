import 'dart:convert';
import 'dart:io';

import 'package:agentapi/agentapi.dart';
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

/// Reads the agent host, port and authentication token from the .address file located at the full path [filepath].
Future<Either<AgentAddrFileError, AuthTarget>> readAgentPortFile(
  String filepath,
) async {
  try {
    final addr = File(filepath);
    // This returns false without crashing even if the [filepath] is invalid.
    if (!await addr.exists()) {
      // error: file doesn't exist.
      return const Left(AgentAddrFileError.nonexistent);
    }

    final contents = await addr.readAsString();
    if (contents.isEmpty) {
      // error: file is empty
      return const Left(AgentAddrFileError.isEmpty);
    }

    return Right(
      AuthTarget.create()..mergeFromProto3Json(jsonDecode(contents)),
      //AuthTarget.fromJson(contents),
    );
  } on FileSystemException catch (_) {
    // error: permission denied
    return const Left(AgentAddrFileError.accessDenied);
  } on FormatException catch (_) {
    // error: format error
    return const Left(AgentAddrFileError.formatError);
  }
}
