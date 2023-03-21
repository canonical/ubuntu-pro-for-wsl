import 'dart:io';

import 'package:logging/logging.dart';

import 'internal/build_extra_targets.dart';
import 'internal/cli_options.dart';
import 'internal/extend_appx_manifest.dart';

Future<void> main(List<String> args) async {
  Logger.root.level = Level.ALL;
  Logger.root.onRecord.listen((record) {
    // ignore: avoid_print
    print('${record.level.name}: ${record.time}: ${record.message}');
  });

  final log = Logger('Pre MSIX Pack');
  final cli = CliOptions.parse(args);

  if (!Directory(cli.buildDir).existsSync()) {
    log.severe(
      'Build directory `${cli.buildDir}` does not exist. Did you run `flutter pub run msix:build` ?',
    );
    return;
  }

  final builder = ExtraTargetsBuilder(
    buildDir: cli.buildDir,
    extraTargetsPath: cli.extraTargetsFile,
  );

  if (!await builder.buildAll(cli.buildDir)) {
    return;
  }

  if (!await extendAppxManifest(cli.buildDir)) {
    return;
  }

  log.info(
    'Success! You can now run `flutter pub run msix:pack` to complete the packaging.',
  );
}
