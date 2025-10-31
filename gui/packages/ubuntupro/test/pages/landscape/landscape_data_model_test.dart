import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import '../../utils/golden.dart';
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
  });
}

void main() {
  group('manual data model', () {
    final testcases = const <String, _ManualTestCase>{
      'success': _ManualTestCase(
        fqdn: selfHostedURL,
      ),
      'success with localhost': _ManualTestCase(
        fqdn: 'localhost',
      ),
      'success with other schemes': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
      ),
      'success with raw ipv4': _ManualTestCase(
        fqdn: '192.168.15.13',
      ),
      'success with raw ipv6': _ManualTestCase(
        fqdn: '2001:db8::1',
      ),
      'ping_url remains http': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
      ),
      'client url remains as input': _ManualTestCase(
        fqdn: 'magnet://$kExampleLandscapeFQDN',
      ),
      'success with full URL': _ManualTestCase(
        fqdn: 'http://$kExampleLandscapeFQDN:8080',
      ),
      'success with registration key': _ManualTestCase(
        fqdn: selfHostedURL,
        registrationKey: 'abc',
      ),
      'success with valid cert': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: validCert,
      ),
      'success with valid cert and key': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: validCert,
        registrationKey: 'abc',
      ),
      'success changing cert into empty path': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: '-',
        registrationKey: 'abc',
      ),
      'success with Landscape docs': _ManualTestCase(
        fqdn: 'landscape-server.domain.com:6555',
      ),
      'error with unintended URIs': _ManualTestCase(
        // This looks like 'host:port' but it's `scheme:path` and we cannot fix it because the path is not an integer.
        fqdn: 'landscape-server.domain.com:6555/',
        wantFQDNError: FqdnError.invalid,
        wantFileError: FileError.none,
        wantComplete: isFalse,
      ),
      'error with SaaS landscape and no account name': _ManualTestCase(
        fqdn: saasURL,
        accountName: '',
        wantAccountNameError: AccountNameError.invalid,
        wantComplete: isFalse,
      ),
      'error with SaaS landscape and standalone account name': _ManualTestCase(
        fqdn: saasURL,
        accountName: standaloneAN,
        wantAccountNameError: AccountNameError.invalid,
        wantComplete: isFalse,
      ),
      'success with SaaS landscape and account name': _ManualTestCase(
        fqdn: saasURL,
        accountName: 'user',
      ),
      'self-hosted rejects not standalone account': _ManualTestCase(
        fqdn: kExampleLandscapeFQDN,
        accountName: 'user',
        wantAccountNameError: AccountNameError.none,
        wantComplete: isTrue,
      ),
      'error with invalid fqdn': _ManualTestCase(
        fqdn: ':::',
        certPath: validCert,
        registrationKey: 'abc',
        wantFQDNError: FqdnError.invalid,
        wantComplete: isFalse,
      ),
      'error due numbers only not being a valid URI': _ManualTestCase(
        fqdn: '12345:6789',
        wantFQDNError: FqdnError.invalid,
        wantComplete: isFalse,
      ),
      'error with not found cert': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: notFoundPath,
        registrationKey: 'abc',
        wantFileError: FileError.notFound,
        wantComplete: isFalse,
      ),
      'error with invalid cert': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: invalidCert,
        registrationKey: 'abc',
        wantFileError: FileError.invalidFormat,
        wantComplete: isFalse,
      ),
      'error with cert path as a dir': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: './test',
        registrationKey: 'abc',
        wantFileError: FileError.dir,
        wantComplete: isFalse,
      ),
      'error with empty cert': _ManualTestCase(
        fqdn: selfHostedURL,
        certPath: emptyFile,
        registrationKey: 'abc',
        wantFileError: FileError.emptyFile,
        wantComplete: isFalse,
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
        if (tc.wantComplete == isTrue) {
          expectGoldenIni(
            'manual_data_model',
            name,
            'landscape.conf',
            c.config()!,
          );
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

const saasURL = 'https://landscape.canonical.com';
const selfHostedURL = 'https://$kExampleLandscapeFQDN';
const customConf = './test/testdata/landscape/custom.conf';
const notFoundPath = './test/testdata/landscape/notfound.txt';
const validCert = './test/testdata/certs/client_cert.pem';
const invalidCert = './test/testdata/certs/not_a_cert.pem';
const emptyFile = './test/testdata/landscape/empty.txt';
