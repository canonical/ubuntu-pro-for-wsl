import 'package:flutter_test/flutter_test.dart';
import 'package:ini/ini.dart';

import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import 'constants.dart';

void main() {
  group('manual data model', () {
    final testcases = {
      'success': (
        fqdn: selfHostedURL,
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with localhost': (
        fqdn: 'localhost',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('localhost:6554'),
      ),
      'success with other schemes': (
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('magnet://$kExampleLandscapeFQDN'),
      ),
      'success with raw ipv4': (
        fqdn: '192.168.15.13',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('https://192.168.15.13/message-system'),
      ),
      'success with raw ipv6': (
        fqdn: '2001:db8::1',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('https://[2001:db8::1]/message-system'),
      ),
      'ping_url remains http': (
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('http://$kExampleLandscapeFQDN/ping'),
      ),
      'client url remains as input': (
        fqdn: 'magnet://$kExampleLandscapeFQDN',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('magnet://$kExampleLandscapeFQDN/message-system'),
      ),
      'success with full URL': (
        fqdn: 'http://$kExampleLandscapeFQDN:8080',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('$kExampleLandscapeFQDN:8080'),
      ),
      'success with registration key': (
        fqdn: selfHostedURL,
        certPath: '',
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with valid cert': (
        fqdn: selfHostedURL,
        certPath: validCert,
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with valid cert and key': (
        fqdn: selfHostedURL,
        certPath: validCert,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success changing cert into empty path': (
        fqdn: selfHostedURL,
        certPath: '-',
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains(kExampleLandscapeFQDN),
      ),
      'success with Landscape docs': (
        fqdn: 'landscape-server.domain.com:6555',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.none,
        wantComplete: isTrue,
        wantConfig: contains('6555'),
      ),
      'error with unintended URIs': (
        // This looks like 'host:port' but it's `scheme:path` and we cannot fix it because the path is not an integer.
        fqdn: 'landscape-server.domain.com:6555/',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.invalid,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with SaaS landscape': (
        fqdn: saasURL,
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.saas,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with invalid fqdn': (
        fqdn: ':::',
        certPath: validCert,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.invalid,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error due numbers only not being a valid URI': (
        fqdn: '12345:6789',
        certPath: '',
        registrationKey: '',
        wantFQDNError: FqdnError.invalid,
        wantFileError: FileError.none,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with not found cert': (
        fqdn: selfHostedURL,
        certPath: notFoundPath,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.notFound,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with invalid cert': (
        fqdn: selfHostedURL,
        certPath: invalidCert,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.invalidFormat,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with cert path as a dir': (
        fqdn: selfHostedURL,
        certPath: './test',
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.dir,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
      'error with empty cert': (
        fqdn: selfHostedURL,
        certPath: emptyFile,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.none,
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
        wantConfig: isNull,
      ),
    };
    for (final MapEntry(key: name, value: tc) in testcases.entries) {
      test(name, () {
        final c = LandscapeManualConfig();
        c.fqdn = tc.fqdn;
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
