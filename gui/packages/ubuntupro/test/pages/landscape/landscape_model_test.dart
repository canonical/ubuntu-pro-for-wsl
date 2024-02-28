import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import 'landscape_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('landscape config', () {
    const pluginChannel = MethodChannelP4wMsStore.methodChannel;
    final pluginMessenger =
        TestWidgetsFlutterBinding.ensureInitialized().defaultBinaryMessenger;
    // Resets the plugin message handler after each test.
    tearDown(() {
      pluginMessenger.setMockMethodCallHandler(pluginChannel, null);
    });

    final client = MockAgentApiClient();

    test('default Landscape configuration', () {
      final model = LandscapeModel(client);
      expect(model.fqdn, '');
      expect(model.accountName, 'standalone');
      expect(model.key, '');
      expect(model.path, '');
    });

    test('no errors by default', () {
      final model = LandscapeModel(client);
      expect(model.fqdnError, false);
      expect(model.fileError, FileError.none);
    });

    test('valid FQDN', () {
      final model = LandscapeModel(client);
      model.fqdn = 'example.com';
      expect(model.validateFQDN(), true);
      expect(model.fqdnError, false);
    });

    test('invalid fqdn', () {
      final model = LandscapeModel(client);
      model.fqdn = '::';
      expect(model.validateFQDN(), false);
      expect(model.fqdnError, true);
    });

    const tempFileName = 'Pro4WSLLandscapeTest.conf';

    test('valid path', () async {
      final model = LandscapeModel(client);
      final tempDir = Directory.systemTemp;
      final tempFile = File('${tempDir.path}/$tempFileName');
      await tempFile.writeAsString('');
      model.path = tempFile.path;
      expect(model.validatePath(), true);
      expect(model.fileError, FileError.none);
      await tempFile.delete();
    });

    test('invalid path', () async {
      final model = LandscapeModel(client);
      final tempDir = Directory.systemTemp;
      final tempFile = File('${tempDir.path}/$tempFileName');
      model.path = tempFile.path;
      expect(model.validatePath(), false);
      expect(model.fileError, FileError.notFound);

      model.path = '';
      expect(model.validatePath(), false);
      expect(model.fileError, FileError.empty);
    });

    test('valid config', () async {
      final model = LandscapeModel(client);
      model.selected = LandscapeConfigType.manual;
      model.fqdn = 'example.com';
      expect(model.validConfig(), true);

      model.selected = LandscapeConfigType.file;
      expect(model.validConfig(), false);
      final tempDir = Directory.systemTemp;
      final tempFile = File('${tempDir.path}/$tempFileName');
      await tempFile.writeAsString('');
      model.path = tempFile.path;
      expect(model.validConfig(), true);

      model.fqdn = '';
      expect(model.validConfig(), true);

      model.selected = LandscapeConfigType.manual;
      expect(model.validConfig(), false);
    });
  });
}
