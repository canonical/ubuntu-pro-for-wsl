import 'dart:io';
import 'package:path/path.dart' as p;

/// Compiles the windows agent's main package into the [destination] directory.
/// Since this is a test helper, assertions are just fine.
/// A try-catch block (on AssertionError) can still prevent process exit.
Future<void> buildAgentExe(String destination) async {
  final dest = Directory(destination);
  await dest.create(recursive: true);

  final goWorkDir = await _findGoWorkspace();
  assert(goWorkDir != null, 'Could not find the go workspace');

  final mainGo = p.normalize(p.join(goWorkDir!.path, _agentPackageDir));

  final result = await Process.run(
    'go',
    ['build', '-ldflags', '-H=windowsgui', mainGo],
    workingDirectory: dest.path,
  );

  assert(result.exitCode == 0, 'go build failed');
}

// The ubuntu-pro-agent's package path relative to the workspace directory.
const _agentPackageDir = 'windows-agent/cmd/ubuntu-pro-agent/';
// The go workspace definition file.
const _goWorkspaceFile = 'go.work';

Future<Directory?> _findGoWorkspace() =>
    _findFileUpwards(startDir: Directory.current, filename: _goWorkspaceFile);

Future<Directory?> _findFileUpwards({
  required String filename,
  required Directory startDir,
}) async {
  if (!await startDir.exists()) {
    return null;
  }

  startDir = startDir.absolute;

  while (true) {
    final file = File(p.join(startDir.path, _goWorkspaceFile));
    if (await file.exists()) {
      return startDir;
    }

    if (startDir.path == startDir.parent.path) {
      return null;
    }

    startDir = startDir.parent;
  }
}
