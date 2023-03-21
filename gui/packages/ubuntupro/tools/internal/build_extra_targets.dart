import 'dart:io';

import 'package:logging/logging.dart';
import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

class Target {
  final String name;
  final String command;
  final List<String> args;
  final String output;
  final String sourceTree;

  Target({
    required this.name,
    required this.command,
    required this.args,
    required this.output,
    required this.sourceTree,
  });
  factory Target.fromYaml(MapEntry<dynamic, dynamic> entry) {
    return Target(
      name: entry.key as String,
      command: entry.value['command'] as String,
      args: (entry.value['args'] as YamlList).toList().cast<String>(),
      output: entry.value['output'] as String,
      sourceTree: entry.value['source-tree'] as String,
    );
  }
  @override
  String toString() {
    return 'Target{name: $name, command: $command, args: $args, output: $output, source-tree: $sourceTree}';
  }
}

class ExtraTargetsBuilder {
  final String buildDir;
  final String extraTargetsPath;
  String? _repoRoot;

  ExtraTargetsBuilder({
    required this.buildDir,
    required this.extraTargetsPath,
  });

  Future<bool> buildAll(String buildDir) async {
    final log = Logger('Extra Build Targets');
    final file = File(extraTargetsPath);

    if (!file.existsSync()) {
      log.warning('No extra targets to build');
      return true;
    }

    for (final t in await _loadBuildTargets(file)) {
      log.info('Building the ${t.name} (${t.output}) with ${t.command}:');
      final res = await _runBuildJob(t, buildDir);

      if (res.exitCode != 0) {
        log.severe(
          'Failed to build the Windows Agent. Refer to the logs below.',
        );
        log.severe(res.stderr);
        log.severe(res.stdout);
        return false;
      }

      if (!await _checkTargetOutput(t)) {
        log.severe('Expected output ${t.output} was not created.');
        return false;
      }
    }

    return true;
  }

  Future<String> _fromRootDir(String relativePath) async {
    _repoRoot ??= await _findRepositoryRootDir();
    return p.absolute(
      p.join(await _findRepositoryRootDir(), p.normalize(relativePath)),
    );
  }

  Future<ProcessResult> _runBuildJob(Target target, String buildDir) async {
    final args = target.args;
    args.add(await _fromRootDir(target.sourceTree));
    final outputDir = p.normalize(p.join(buildDir, p.dirname(target.output)));
    await Directory(outputDir).create();
    return Process.run(target.command, args, workingDirectory: outputDir);
  }

  Future<bool> _checkTargetOutput(Target target) {
    return File(p.normalize(p.join(buildDir, target.output))).exists();
  }
}

Future<Iterable<Target>> _loadBuildTargets(File file) async {
  final targets = loadYamlDocument(await file.readAsString());
  final contents = targets.contents as YamlMap;
  return contents.entries.map(Target.fromYaml);
}

Future<String> _findRepositoryRootDir() async {
  const rootElements = ['.git', '.github'];
  var current = Directory.current;
  while (true) {
    final isRootDir = await current.list().any(
          (el) => rootElements.contains(p.basename(el.path)),
        );

    if (isRootDir) {
      return current.path;
    }

    if (current == current.parent) {
      throw const FileSystemException('This seems not to be a git repository');
    }

    current = current.parent;
  }
}
