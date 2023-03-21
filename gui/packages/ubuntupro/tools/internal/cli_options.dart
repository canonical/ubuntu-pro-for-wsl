import 'package:args/args.dart';

const _buildDir = 'build/windows/runner/Release/';
const _extraTargetsFile = 'tools/extra_build_targets.yaml';

class CliOptions {
  final String buildDir;
  final String extraTargetsFile;

  CliOptions(this.buildDir, this.extraTargetsFile);

  factory CliOptions.parse(List<String> args) {
    final parser = ArgParser();
    parser.addOption('build-dir', abbr: 'b', defaultsTo: _buildDir);
    parser.addOption(
      'extra-targets-file',
      abbr: 't',
      defaultsTo: _extraTargetsFile,
    );

    final options = parser.parse(args);
    return CliOptions(
      options['build-dir'] as String,
      options['extra-targets-file'] as String,
    );
  }
}
