import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:ini/ini.dart';
import 'package:path/path.dart' as p;

void expectRawGolden(
    String group, String testname, String goldenName, String actual) {
  final want = _loadGolden(group, testname, goldenName, actual);

  expect(actual, want);
}

void expectGoldenIni(
    String group, String testname, String goldenName, String actual) {
  final golden = _loadGolden(group, testname, goldenName, actual);
  final want = Config.fromString(golden);
  final got = Config.fromString(actual);

  for (final wantSection in want.sections()) {
    expect(got.sections(), contains(wantSection));
    final wantItems = want.items(wantSection)!;
    final gotItems = got.items(wantSection)!;
    for (final wantItem in wantItems) {
      final gotItem = gotItems.firstWhere(
          (element) => element[0] == wantItem[0],
          orElse: () => throw TestFailure(
              'Missing item [${wantItem[0]}] in section [$wantSection]'));
      expect(gotItem[1], wantItem[1],
          reason:
              'Mismatch for item [${wantItem[0]}] in section [$wantSection]');
    }
  }
}

String _loadGolden(
    String group, String testname, String goldenName, String actual) {
  final goldenPath = _testDir(group, testname);
  final goldenFile = p.join(goldenPath, goldenName);
  final golden = File(goldenFile);

  final mustUpdateGoldens = Platform.environment['TESTS_UPDATE_GOLDEN'];

  if (['1', 'true', 'True'].contains(mustUpdateGoldens)) {
    golden.createSync(recursive: true);
    golden.writeAsStringSync(actual);
    return actual;
  }

  return golden.readAsStringSync();
}

String _testDir(String group, String name) {
  final testDir = (goldenFileComparator as LocalFileComparator).basedir;

  return p.join(
    'test',
    testDir.toFilePath(windows: Platform.isWindows),
    'golden',
    group.replaceAll(RegExp(r'[ <>:"/\\|?*]', multiLine: true), '_'),
    name.replaceAll(RegExp(r'[ <>:"/\\|?*]', multiLine: true), '_'),
  );
}
