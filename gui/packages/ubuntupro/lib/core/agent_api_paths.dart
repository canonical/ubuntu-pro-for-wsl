import 'dart:io';
import 'package:path/path.dart' as p;

/// Provides the full path of the "[appDir]/[filename]" file
/// under the well known directory where the Windows Agent stores its local data.
String agentAddrFilePath(String appDir, String filename) {
// The well-known package path_provider doesn't return the LOCALAPPDATA directory
// but the APPDATA, which is usually under %HOME%/AppData/Roaming instead of
// %HOME%/AppData/Local, which is where the agent is storing the support data.
  final localAppDir = Platform.environment['LOCALAPPDATA'];
  return p.join(localAppDir!, appDir, filename);
}

/// Reads the agent port from the addr file located at the full path [filepath].
Future<int> readAgentPortFromFile(String filepath) async {
  final addr = File(filepath);
  final lines = await addr.readAsLines();
  return readAgentPortFromLine(lines[0]);
}

/// Parses [line] assuming it's from Windows Agent addr file.
int readAgentPortFromLine(String line) => int.parse(line.split(':').last);
