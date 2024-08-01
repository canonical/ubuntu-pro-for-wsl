import 'package:flutter_test/flutter_test.dart';
import 'package:ini/ini.dart';

import 'package:ubuntupro/pages/landscape/landscape_model.dart';

void main() {
  group('saas data model', () {
    final testcases = {
      'success': (
        account: 'test',
        wantError: isFalse,
        wantComplete: isTrue,
        wantConfig: contains('landscape.canonical.com')
      ),
      'with empty account': (
        account: '',
        wantError: isFalse,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with account standalone': (
        account: 'standalone',
        wantError: isTrue,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
    };
    for (final MapEntry(key: name, value: tc) in testcases.entries) {
      test(name, () {
        final c = LandscapeSaasConfig();
        c.accountName = tc.account;
        expect(c.accountNameError, tc.wantError);
        expect(c.isComplete, tc.wantComplete);
        final raw = c.config();
        expect(raw, tc.wantConfig);
        if (raw != null) {
          expectNoEmptyValuesInINI(raw);
        }
      });
    }
  });
  group('self-hosted data model', () {
    const testUrl = 'test.landscape.company.com';
    final testcases = {
      'success': (
        url: testUrl,
        certPath: '',
        wantFqdnError: isFalse,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(testUrl)
      ),
      'with SaaS URL': (
        url: saasURL,
        certPath: '',
        wantFqdnError: isTrue,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with SaaS hostname': (
        url: Uri.parse(saasURL).host,
        certPath: '',
        wantFqdnError: isTrue,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with ssl key path as dir': (
        url: testUrl,
        // the directory that contains the test sources.
        certPath: './test',
        wantFqdnError: isFalse,
        wantFileError: FileError.dir,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with ssl key changing into empty path': (
        url: testUrl,
        // Magic value to make the test case apply a good path first, then an empty path.
        certPath: '-',
        wantFqdnError: isFalse,
        // SSL key path is an optional entry.
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(testUrl),
      ),
      'with ssl key file empty': (
        url: testUrl,
        certPath: './test/testdata/landscape/empty.txt',
        wantFqdnError: isFalse,
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with ssl key not found': (
        url: testUrl,
        certPath: notFoundPath,
        wantFqdnError: isFalse,
        wantFileError: FileError.notFound,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
    };
    for (final MapEntry(key: name, value: tc) in testcases.entries) {
      test(name, () {
        final c = LandscapeSelfHostedConfig();
        c.fqdn = tc.url;

        // Dart records can't be modified, so we need a proxy variable.
        var path = tc.certPath;
        if (tc.certPath == '-') {
          // Apply a good path first.
          c.sslKeyPath = customConf;
          path = '';
        }
        c.sslKeyPath = path;

        expect(c.fqdnError, tc.wantFqdnError);
        expect(c.fileError, tc.wantFileError);
        expect(c.isComplete, tc.wantComplete);
        final raw = c.config();
        expect(raw, tc.wantConfig);
        if (raw != null) {
          expectNoEmptyValuesInINI(raw);
        }
      });
    }
  });
  group('custom data model', () {
    final testcases = {
      'success': (
        path: customConf,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('some.url.com'),
      ),
      'with config file not found': (
        path: notFoundPath,
        wantFileError: FileError.notFound,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with empty path': (
        path: '',
        wantFileError: FileError.emptyPath,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with empty config file': (
        path: './test/testdata/landscape/empty.txt',
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with config file too large': (
        // a big file (1.5 MB) always present when running tests.
        path: './build/unit_test_assets/fonts/MaterialIcons-Regular.otf',
        wantFileError: FileError.tooLarge,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
      'with config file as dir': (
        // a big file (1.5 MB) always present when running tests.
        path: './test/',
        wantFileError: FileError.dir,
        wantComplete: isFalse,
        wantConfig: isNull
      ),
    };

    for (final MapEntry(key: name, value: tc) in testcases.entries) {
      test(name, () async {
        final c = LandscapeCustomConfig();
        if (tc.path.isEmpty) {
          // Applying an empty path is only a problem if a previous value was not empty,
          // otherwise the UI would show error messages from start.
          c.configPath = customConf;
        }
        c.configPath = tc.path;
        expect(c.fileError, tc.wantFileError);
        expect(c.isComplete, tc.wantComplete);
        expect(c.config(), tc.wantConfig);
      });
    }
  });
}

void expectNoEmptyValuesInINI(String raw) {
  final config = Config.fromStrings(raw.split('\n'));
  for (final o in config.items('client')!) {
    expect(o[1], isNotEmpty);
  }
}

const saasURL = 'https://landscape.canonical.com';
const customConf = './test/testdata/landscape/custom.conf';
const notFoundPath = './test/testdata/landscape/notfound.txt';
