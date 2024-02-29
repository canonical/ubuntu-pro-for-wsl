import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import 'landscape_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('landscape config', () {
    final client = MockAgentApiClient();
    final tempDir = Directory.systemTemp;
    const tempFileName = 'Pro4WSLLandscapeTEMP.conf';
    final tempFilePath = '${tempDir.path}/$tempFileName';

    tearDown(() async {
      final file = File(tempFilePath);
      if (await file.exists()) {
        await file.delete();
      }
    });

    test('default Landscape configuration', () {
      final model = LandscapeModel(client);
      expect(model.fqdn, '');
      expect(model.accountName, 'standalone');
      expect(model.key, '');
      expect(model.path, '');
    });

    test('no errors by default', () {
      final model = LandscapeModel(client);
      expect(model.fqdnError, isFalse);
      expect(model.fileError, FileError.none);
      expect(model.receivedInput, isFalse);
      expect(model.hasError, isTrue);
    });

    test('valid FQDN', () {
      final model = LandscapeModel(client);
      model.fqdn = 'example.com';
      expect(model.validateFQDN(), isTrue);
      expect(model.fqdnError, isFalse);
      expect(model.fqdn, 'https://example.com');
      model.fqdn = 'https://anotherexample.com';
      expect(model.validateFQDN(), isTrue);
      expect(model.fqdnError, isFalse);
      expect(model.fqdn, 'https://anotherexample.com');
    });

    test('invalid fqdn', () {
      final model = LandscapeModel(client);
      model.fqdn = '::';
      expect(model.validateFQDN(), isFalse);
      expect(model.fqdnError, isTrue);
    });

    test('valid path', () async {
      final model = LandscapeModel(client);
      final tempFile = File(tempFilePath);
      await tempFile.writeAsString('');
      model.path = tempFile.path;
      expect(model.validatePath(), isTrue);
      expect(model.fileError, FileError.none);
      await tempFile.delete();
    });

    test('invalid path', () async {
      final model = LandscapeModel(client);
      final tempFile = File(tempFilePath);
      model.path = tempFile.path;
      expect(model.validatePath(), isFalse);
      expect(model.fileError, FileError.notFound);

      model.path = tempFile.path;
      await tempFile.writeAsString('.' * (1024 * 1024 + 1));
      expect(model.validatePath(), isFalse);
      expect(model.fileError, FileError.tooLarge);

      model.path = '';
      expect(model.validatePath(), isFalse);
      expect(model.fileError, FileError.empty);
    });

    test('valid config', () async {
      final model = LandscapeModel(client);
      model.selected = LandscapeConfigType.manual;
      model.fqdn = 'example.com';
      expect(model.validConfig(), isTrue);

      model.selected = LandscapeConfigType.file;
      expect(model.validConfig(), isFalse);
      final tempFile = File(tempFilePath);
      await tempFile.writeAsString('');
      model.path = tempFile.path;
      expect(model.validConfig(), isTrue);

      model.fqdn = '';
      expect(model.validConfig(), isTrue);

      model.selected = LandscapeConfigType.manual;
      expect(model.validConfig(), isFalse);
    });

    test('valid apply', () async {
      var model = LandscapeModel(client);
      model.fqdn = 'example.com';
      expect(await model.applyConfig(), isTrue);

      model = LandscapeModel(client);
      model.selected = LandscapeConfigType.file;
      final tempFile = File(tempFilePath);
      await tempFile.writeAsString('');
      model.path = tempFile.path;
      expect(await model.applyConfig(), isTrue);
    });

    test('invalid apply', () async {
      final model = LandscapeModel(client);
      expect(await model.applyConfig(), isFalse);
    });
  });
}
