import 'dart:io';

import 'package:agentapi/agentapi.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';

import 'constants.dart';
import 'landscape_model_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  group('state management', () {
    final client = MockAgentApiClient();
    test('active', () {
      final model = LandscapeModel(client);
      expect(model.configType, LandscapeConfigType.manual);

      for (final type in LandscapeConfigType.values) {
        model.setConfigType(type);
        expect(model.configType, type);
      }
    });

    test('notify changes', () {
      // Verifies that all set* methods notify listeners.

      final model = LandscapeModel(client);
      var notified = false;
      model.addListener(() {
        notified = true;
      });

      model.setConfigType(LandscapeConfigType.custom);
      expect(notified, isTrue);
      notified = false;

      model.setCustomConfigPath(customConf);
      expect(notified, isTrue);
      notified = false;

      model.setConfigType(LandscapeConfigType.manual);
      expect(notified, isTrue);
      notified = false;

      model.setManualRegistrationKey('123');
      expect(notified, isTrue);
      notified = false;

      model.setFqdn('https://example.com');
      expect(notified, isTrue);
      notified = false;

      model.setSslKeyPath('/some/path');
      expect(notified, isTrue);
      notified = false;
    });

    test('assertions', () {
      // Verifies that methods throw assertions when called under non-relevant scenarios.
      // Those assertions exist because the methods are not relevant for the current config type.
      // Allowing those conditions to proceed could contribute to hide logic errors.

      final model = LandscapeModel(client);

      model.setConfigType(LandscapeConfigType.manual);
      expect(() => model.setCustomConfigPath(customConf), throwsAssertionError);

      model.setConfigType(LandscapeConfigType.custom);
      expect(() => model.setSslKeyPath(customConf), throwsAssertionError);
      expect(() => model.setManualRegistrationKey('123'), throwsAssertionError);
      expect(() => model.setFqdn(testFqdn), throwsAssertionError);
      expect(() => model.setManualRegistrationKey('123'), throwsAssertionError);
      expect(() => model.setSslKeyPath(customConf), throwsAssertionError);
    });
  });

  group('apply config', () {
    const msg = 'test message';
    const error = GrpcError.custom(StatusCode.unavailable, msg);
    test('manual', () async {
      final client = MockAgentApiClient();
      when(
        client.applyLandscapeConfig(any),
      ).thenAnswer((_) async => throw error);
      final model = LandscapeModel(client);

      model.setConfigType(LandscapeConfigType.manual);
      expect(model.applyConfig, throwsAssertionError);

      model.setFqdn(kExampleLandscapeFQDN);
      var err = await model.applyConfig();
      expect(err.message, msg);

      when(
        client.applyLandscapeConfig(any),
      ).thenAnswer((_) async => LandscapeSource()..ensureUser());

      model.setFqdn(kExampleLandscapeFQDN);
      err = await model.applyConfig();
      expect(err.message, isNull);

      model.setSslKeyPath(caCert);
      err = await model.applyConfig();
      expect(err.message, isNull);

      model.setManualRegistrationKey('abc');
      err = await model.applyConfig();
      expect(err.message, isNull);
    });

    test('custom', () async {
      final client = MockAgentApiClient();
      when(
        client.applyLandscapeConfig(any),
      ).thenAnswer((_) async => throw error);
      final model = LandscapeModel(client);

      model.setConfigType(LandscapeConfigType.custom);
      expect(model.applyConfig, throwsAssertionError);

      model.setCustomConfigPath(customConf);
      var err = await model.applyConfig();
      expect(err.message, msg);

      when(
        client.applyLandscapeConfig(any),
      ).thenAnswer((_) async => LandscapeSource()..ensureUser());

      model.setCustomConfigPath(customConf);
      err = await model.applyConfig();
      expect(err.message, isNull);
    });
  });

  test('apply config errors', () async {
    const msg = 'test message';
    const errors = <Exception>[
      // The ones we do something about
      GrpcError.alreadyExists(msg),
      GrpcError.invalidArgument(msg),
      GrpcError.permissionDenied(msg),
      GrpcError.unavailable(msg),

      /// Some we don't
      GrpcError.deadlineExceeded(msg),
      GrpcError.unknown(msg),

      /// And finally some non-gRPC related error:
      FileSystemException(msg),
    ];

    const expectedCodes = [
      StatusCode.alreadyExists,
      StatusCode.invalidArgument,
      StatusCode.permissionDenied,
      StatusCode.unavailable,
      StatusCode.deadlineExceeded,
      StatusCode.unknown,
      StatusCode.unknown,
    ];

    final client = MockAgentApiClient();
    final model = LandscapeModel(client);

    model.setConfigType(LandscapeConfigType.manual);
    model.setFqdn(kExampleLandscapeFQDN);

    for (var i = 0; i < errors.length; i++) {
      when(
        client.applyLandscapeConfig(any),
      ).thenAnswer((_) async => throw errors[i]);

      final got = await model.applyConfig();
      expect(got.code, expectedCodes[i]);
    }
  });
}

const customConf = './test/testdata/landscape/custom.conf';
const saasURL = 'https://landscape.canonical.com';
const testFqdn = 'test.landscape.company.com';
const caCert = './test/testdata/certs/ca_cert.pem';
