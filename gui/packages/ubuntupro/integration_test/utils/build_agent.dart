import 'dart:io';
import 'package:path/path.dart' as p;

/// Compiles the windows agent's main package into the [destination] directory,
/// optionally renaming it to [exeName].
/// Since this is a test helper, assertions are just fine.
/// A try-catch block (on AssertionError) can still prevent process exit.
Future<void> buildAgentExe(
  String destination, {
  String exeName = 'ubuntu-pro-agent-launcher.exe',
}) async {
  const config = 'Debug';
  const platform = 'x64';

  final dest = Directory(destination);
  await dest.create(recursive: true);

  // <...>/msix/agent/agent.vcxproj
  final vcxproj = await _findAgentVcxproj();
  assert(vcxproj != null, 'Could not find the agent project');

  await _build(
    buildProgram: 'msbuild',
    targetPath: vcxproj!,
    arguments: [
      '/p:Configuration=$config',
      '/p:Platform=$platform',
      '/p:GoBuildTags=-tags="gowslmock,integrationtests"',
    ],
  );

  // <...>/msix/agent/x64/Debug/ubuntu-pro-agent.exe
  final expectedOutput = p.join(
    p.dirname(vcxproj),
    platform,
    config,
    'ubuntu-pro-agent.exe',
  );

  await File(expectedOutput).rename(p.join(dest.absolute.path, exeName));
}

Future<void> _build({
  required String buildProgram,
  List<String>? arguments,
  required String targetPath,
  String? workingDir,
}) async {
  final result = await Process.run(buildProgram, [
    ...?arguments,
    targetPath,
  ], workingDirectory: workingDir);

  stdout.write(result.stdout);
  stdout.write(result.stderr);

  assert(
    result.exitCode == 0,
    '$buildProgram failed:\n${result.stderr}\n${result.stdout}',
  );
}

Future<String?> _findAgentVcxproj() => _findFileUpwards(
  startDir: Directory.current,
  name: 'msix/agent/agent.vcxproj',
);

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
