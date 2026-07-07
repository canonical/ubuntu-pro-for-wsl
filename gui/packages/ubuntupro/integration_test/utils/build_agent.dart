import 'dart:io';
import 'package:path/path.dart' as p;

/// Compiles the windows agent's main package into the [destination] directory,
/// optionally renaming it to [exeName].
Future<void> buildAgentExe(
  String destination, {
  String exeName = 'ubuntu-pro-agent-launcher.exe',
}) async {
  final dest = Directory(destination);
  await dest.create(recursive: true);

  final root = await _findWorkspaceRoot();
  assert(root != null, 'Could not find workspace root');
  final agentDir = p.join(root!, 'windows-agent', 'cmd', 'ubuntu-pro-agent');

  final result = await Process.run(
    'go',
    [
      'build',
      '-tags=gowslmock,integrationtests',
      '-o',
      p.join(dest.absolute.path, exeName),
      '.',
    ],
    workingDirectory: agentDir,
  );

  stdout.write(result.stdout);
  stderr.write(result.stderr);
  assert(result.exitCode == 0,
      'go build failed:\n${result.stderr}\n${result.stdout}');
}

Future<String?> _findWorkspaceRoot() async {
  final path = await _findFileUpwards(
    startDir: Directory.current,
    name: 'go.work',
  );
  return path != null ? p.dirname(path) : null;
}

/// Iterates upwards from [startDir] looking for a file matching the join "[startDir]/[name]".
Future<String?> _findFileUpwards({
  required String name,
  required Directory startDir,
}) async {
  if (!await startDir.exists()) {
    return null;
  }

  startDir = startDir.absolute;

  while (true) {
    final file = File(p.join(startDir.path, name));
    if (await file.exists()) {
      return file.path;
    }

    if (startDir.path == startDir.parent.path) {
      return null;
    }

    startDir = startDir.parent;
  }
}
