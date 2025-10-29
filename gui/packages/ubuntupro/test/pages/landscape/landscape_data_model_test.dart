import 'package:flutter_test/flutter_test.dart';
import 'package:ini/ini.dart';

import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import 'constants.dart';

// Records are great when fields have no meaningful default values.
class _ManualTestCase {
  // inputs
  final String fqdn;
  final String accountName;
  final String certPath;
  final String registrationKey;

  // expected outputs
  final FqdnError wantFQDNError;
  final FileError wantFileError;
  final AccountNameError wantAccountNameError;
  final Matcher wantComplete;
  final Matcher wantConfig;

  const _ManualTestCase({
    // inputs
    required this.fqdn,
    this.accountName = standaloneAN,
    this.certPath = '',
    this.registrationKey = '',

    // expected outputs
    this.wantFQDNError = FqdnError.none,
    this.wantFileError = FileError.none,
    this.wantAccountNameError = AccountNameError.none,
    this.wantComplete = isTrue,
    required this.wantConfig,
  });
}

void main() {
  group('manual data model', () {
    final testcases = <String, _ManualTestCase>{
      'success': _ManualTestCase(
        fqdn: selfHostedURL,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with localhost': _ManualTestCase(
        fqdn: 'localhost',
        wantConfig: contains('localhost:6554'),
      ),
      'success with other schemes': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        wantConfig: contains('magnet://$kExampleLandscapeFQDN'),
      ),
      'success with raw ipv4': _ManualTestCase(
        fqdn: '192.168.15.13',
        wantConfig: contains('https://192.168.15.13/message-system'),
      ),
      'success with raw ipv6': _ManualTestCase(
        fqdn: '2001:db8::1',
        wantConfig: contains('https://[2001:db8::1]/message-system'),
      ),
      'ping_url remains http': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        wantConfig: contains('http://$kExampleLandscapeFQDN/ping'),
      ),
      'client url remains as input': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        wantConfig: contains('magnet://$kExampleLandscapeFQDN/message-system'),
      ),
      'success with full URL': _ManualTestCase(
        fqdn: 'http://$kExampleLandscapeFQDN:8080',
        wantConfig: contains('$kExampleLandscapeFQDN:8080'),
      ),
      'success with registration key': _ManualTestCase(
        fqdn: selfHostedURL,
        registrationKey: 'abc',
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with valid cert': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: validCert,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with valid cert and key': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: validCert,
        registrationKey: 'abc',
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success changing cert into empty path': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: '-',
        registrationKey: 'abc',
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with Landscape docs': _ManualTestCase(
        fqdn: 'landscape-server.domain.com:6555',
        wantConfig: contains('6555'),
      ),
      'error with unintended URIs': const _ManualTestCase(
        // This looks like 'host:port' but it's `scheme:path` and we cannot fix it because the path is not an integer.
        fqdn: 'landscape-server.domain.com:6555/',
        wantFQDNError: FqdnError.invalid,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with SaaS landscape and no account name': const _ManualTestCase(
        fqdn: saasURL,
        accountName: '',
        wantAccountNameError: AccountNameError.invalid,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with SaaS landscape and standalone account name':
          const _ManualTestCase(
        fqdn: saasURL,
        accountName: standaloneAN,
        wantAccountNameError: AccountNameError.invalid,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'success with SaaS landscape and account name': _ManualTestCase(
        fqdn: saasURL,
        accountName: 'user',
        wantConfig: contains('user'),
      ),
      'self-hosted rejects not standalone account': _ManualTestCase(
        fqdn: kExampleLandscapeFQDN,
        accountName: 'user',
        wantAccountNameError: AccountNameError.none,
        wantComplete: isTrue,
        wantConfig: contains('account_name = standalone'),
      ),
      'error with invalid fqdn': const _ManualTestCase(
        fqdn: ':::',
        certPath: validCert,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.invalid,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error due numbers only not being a valid URI': const _ManualTestCase(
        fqdn: '12345:6789',
        wantFQDNError: FqdnError.invalid,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with not found cert': const _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: notFoundPath,
        registrationKey: 'abc',
        wantFileError: FileError.notFound,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with invalid cert': const _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: invalidCert,
        registrationKey: 'abc',
        wantFileError: FileError.invalidFormat,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with cert path as a dir': const _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: './test',
        registrationKey: 'abc',
        wantFileError: FileError.dir,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with empty cert': const _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: emptyFile,
        registrationKey: 'abc',
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
    };
    for (final MapEntry(key: name, value: tc) in testcases.entries) {
      test(name, () {
        final c = LandscapeManualConfig();
        c.fqdn = tc.fqdn;
        c.accountName = tc.accountName;
        c.registrationKey = tc.registrationKey;

        var path = tc.certPath;
        if (tc.certPath == '-') {
          // Apply a good path first.
          c.sslKeyPath = validCert;
          path = '';
        }
        c.sslKeyPath = path;

        expect(c.fqdnError, tc.wantFQDNError);
        expect(c.fileError, tc.wantFileError);
        expect(c.accountNameError, tc.wantAccountNameError);
        expect(c.isComplete, tc.wantComplete);
        final raw = c.config();
        expect(raw, tc.wantConfig);
        if (raw != null) {
          expectINI(raw);
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
        wantConfig: isNull,
      ),
      'with empty path': (
        path: '',
        wantFileError: FileError.emptyPath,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'with empty config file': (
        path: emptyFile,
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'with config file too large': (
        // a big file (1.5 MB) always present when running tests.
        path: './build/unit_test_assets/fonts/MaterialIcons-Regular.otf',
        wantFileError: FileError.tooLarge,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'with config file as dir': (
        // a big file (1.5 MB) always present when running tests.
        path: './test/',
        wantFileError: FileError.dir,
        wantComplete: isFalse,
        wantConfig: isNull,
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

void expectINI(String raw) {
  final config = Config.fromStrings(raw.split('\n'));
  expectNoEmptyValuesInINI(config);
  expectUrlSchemes(config);
}

void expectNoEmptyValuesInINI(Config config) {
  for (final o in config.items('client')!) {
    expect(o[1], isNotEmpty);
  }
}

void expectUrlSchemes(Config config) {
  final ping = Uri.parse(config.get('client', 'ping_url')!);
  expect(ping.scheme, 'http');
}

const saasURL = 'https://landscape.canonical.com';
const selfHostedURL = 'https://$kExampleLandscapeFQDN';
const customConf = './test/testdata/landscape/custom.conf';
const notFoundPath = './test/testdata/landscape/notfound.txt';
const validCert = './test/testdata/certs/client_cert.pem';
const invalidCert = './test/testdata/certs/not_a_cert.pem';
const emptyFile = './test/testdata/landscape/empty.txt';
