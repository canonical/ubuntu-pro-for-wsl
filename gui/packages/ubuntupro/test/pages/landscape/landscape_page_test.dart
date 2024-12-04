import 'package:agentapi/agentapi.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';
import 'package:ubuntupro/pages/landscape/landscape_page.dart';
import 'package:ubuntupro/pages/widgets/page_widgets.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';
import 'package:yaru_test/yaru_test.dart';

import '../../utils/build_multiprovider_app.dart';
import 'landscape_page_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  // TODO: Sometimes the Column in the LandscapePage extends past the test environment's screen
  // due differences in font size between production and testing environments.
  // This should be resolved so that we don't have to specify a manual text scale factor.
  // See more: https://github.com/flutter/flutter/issues/108726#issuecomment-1205035859
  binding.platformDispatcher.textScaleFactorTestValue = 0.6;
  FilePicker.platform = FakeFilePicker([caCert]);

  group('input sections', () {
    testWidgets('default state', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<ElevatedButton>(continueButton).enabled, isFalse);

      for (final type in LandscapeConfigType.values) {
        final radio = find.byWidgetPredicate(
          (widget) => widget is YaruRadio && widget.value == type,
        );
        expect(radio, findsOne);
        expect(
          tester.widget<YaruRadio>(radio).groupValue ==
              LandscapeConfigType.selfHosted,
          isTrue,
        );
      }
    });

    testWidgets('continue enabled', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());
      model.setConfigType(LandscapeConfigType.saas);
      model.setAccountName('testaccount');

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<ElevatedButton>(continueButton).enabled, isTrue);
    });
  });

  group('calls back on success', () {
    testWidgets('saas', (tester) async {
      final agent = MockAgentApiClient();
      final model = LandscapeModel(agent);

      var applied = false;
      when(agent.applyLandscapeConfig(any)).thenAnswer((_) async {
        applied = true;
        return LandscapeSource()..ensureUser();
      });

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final saasRadio = find.ancestor(
        of: find.text(lang.landscapeQuickSetupSaas),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(saasRadio);
      await tester.pump();

      final accountInput = find.ancestor(
        of: find.text(lang.landscapeAccountNameLabel),
        matching: find.byType(TextField),
      );
      final continueButton = find.button(lang.buttonNext);

      // expect false since account name cannot be 'standalone' for the saas subform.
      await tester.enterText(accountInput, standalone);
      await tester.pump();
      await tester.tap(continueButton);
      expect(applied, isFalse);

      await tester.enterText(accountInput, 'testaccount');
      await tester.pump();
      await tester.tap(continueButton);
      await tester.pump();
      expect(applied, isTrue);
    });
    testWidgets('self-hosted', (tester) async {
      final agent = MockAgentApiClient();
      final model = LandscapeModel(agent);

      var applied = false;
      when(agent.applyLandscapeConfig(any)).thenAnswer((_) async {
        applied = true;
        return LandscapeSource()..ensureUser();
      });

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final selfHosted = find.ancestor(
        of: find.text(lang.landscapeQuickSetupSelfHosted),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(selfHosted);
      await tester.pump();
      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFQDNLabel),
        matching: find.byType(TextField),
      );
      final continueButton = find.button(lang.buttonNext);

      // expect false since FQDN cannot be landscapeSaas for the self-hosted subform.
      await tester.enterText(fqdnInput, landscapeSaas);
      await tester.pump();
      await tester.tap(continueButton);
      expect(applied, isFalse);

      await tester.enterText(fqdnInput, 'test.l.com');
      await tester.pump();

      await tester.tap(find.text(lang.landscapeFilePicker));
      await tester.pumpAndSettle();

      final fileInput = find.ancestor(
        of: find.text(caCert),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);

      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, clientCert);
      await tester.pump();

      await tester.tap(continueButton);
      await tester.pump();
      expect(applied, isTrue);
    });

    testWidgets('custom config', (tester) async {
      final client = MockAgentApiClient();

      var applied = false;
      when(client.applyLandscapeConfig(any)).thenAnswer((_) async {
        applied = true;
        return LandscapeSource()..ensureUser();
      });

      final model = LandscapeModel(client);
      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final customRadio = find.ancestor(
        of: find.text(lang.landscapeCustomSetup),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(customRadio);
      await tester.pump();

      final fileInput = find.ancestor(
        of: find.text(lang.landscapeFileLabel),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);
      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, customConf);
      await tester.pump();

      final continueButton = find.button(lang.buttonNext);
      expect(tester.widget<ElevatedButton>(continueButton).enabled, isTrue);
      expect(applied, isFalse);

      await tester.tap(continueButton);
      await tester.pumpAndSettle();
      expect(applied, isTrue);
    });
  });

  group('feedback on error', () {
    testWidgets('saas', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final saasRadio = find.ancestor(
        of: find.text(lang.landscapeQuickSetupSaas),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(saasRadio);
      await tester.pump();

      final accountInput = find.ancestor(
        of: find.text(lang.landscapeAccountNameLabel),
        matching: find.byType(TextField),
      );
      expect(accountInput, findsOne);

      await tester.enterText(accountInput, standalone);
      await tester.pump();

      final errorText = find.text(lang.landscapeAccountNameError);
      expect(errorText, findsOne);
    });
    testWidgets('self-hosted', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final selfHosted = find.ancestor(
        of: find.text(lang.landscapeQuickSetupSelfHosted),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(selfHosted);
      await tester.pump();

      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFQDNLabel),
        matching: find.byType(TextField),
      );
      expect(fqdnInput, findsOne);

      await tester.enterText(fqdnInput, '::');
      await tester.pump();

      final fqdnErrorText = find.text(lang.landscapeFQDNError);
      expect(fqdnErrorText, findsOne);

      await tester.tap(find.text(lang.landscapeFilePicker));
      await tester.pumpAndSettle();

      final fileInput = find.ancestor(
        of: find.text(lang.landscapeSSLKeyLabel),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);

      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, notFoundPath);
      await tester.pump();

      final fileErrorText = find.text(lang.landscapeFileNotFound);
      expect(fileErrorText, findsOne);

      await tester.enterText(fileInput, clientKey);
      await tester.pump();
      expect(find.text(lang.landscapeFileInvalidFormat), findsOne);

      await tester.enterText(fileInput, notATextCert);
      await tester.pump();
      expect(find.text(lang.landscapeFileInvalidFormat), findsOne);

      await tester.enterText(fileInput, notABinCert);
      await tester.pump();
      expect(find.text(lang.landscapeFileInvalidFormat), findsOne);

      await tester.enterText(fileInput, binaryCert);
      await tester.pump();
      expect(find.text(lang.landscapeFileInvalidFormat), findsNothing);
    });

    testWidgets('custom config', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final customRadio = find.ancestor(
        of: find.text(lang.landscapeCustomSetup),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(customRadio);
      await tester.pump();
      final fileInput = find.ancestor(
        of: find.text(lang.landscapeFileLabel),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);
      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, notFoundPath);
      await tester.pump();

      final errorText = find.text(lang.landscapeFileNotFound);
      expect(errorText, findsOne);
    });

    testWidgets('on agent error', (tester) async {
      final client = MockAgentApiClient();
      const msg = 'agent error message';
      const err = GrpcError.custom(17, msg);
      when(client.applyLandscapeConfig(any)).thenThrow(err);
      final model = LandscapeModel(client);

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(LandingPage));
      final lang = AppLocalizations.of(context);

      final saasRadio = find.ancestor(
        of: find.text(lang.landscapeQuickSetupSaas),
        matching: find.byType(YaruSelectableContainer),
      );
      await tester.tap(saasRadio);
      await tester.pump();
      final accountInput = find.ancestor(
        of: find.text(lang.landscapeAccountNameLabel),
        matching: find.byType(TextField),
      );
      expect(accountInput, findsOne);
      await tester.tap(accountInput);
      await tester.pump();
      await tester.enterText(accountInput, 'testaccount');
      await tester.pump();

      final next = find.button(lang.buttonNext);
      await tester.tap(next);
      await tester.pump();
      final snack = find.descendant(
        of: find.byType(SnackBar),
        matching: find.byType(Text),
      );

      expect(snack, findsOne);
      expect(tester.widget<Text>(snack).data, contains(msg));
    });
  });

  group('create', () {
    final mockClient = MockAgentApiClient();
    registerServiceInstance<AgentApiClient>(mockClient);

    for (final late in [true, false]) {
      testWidgets('is late: $late', (tester) async {
        final app = buildMultiProviderWizardApp(
          routes: {
            '/': WizardRoute(
              builder: (ctx) => LandscapePage.create(ctx, isLate: late),
            ),
          },
        );

        await tester.pumpWidget(app);
        await tester.pumpAndSettle();

        final context = tester.element(find.byType(LandscapePage));
        final model = Provider.of<LandscapeModel>(context, listen: false);

        expect(model, isNotNull);
      });
    }
  });
}

Widget buildApp(
  LandscapeModel model,
) {
  return buildSingleRouteMultiProviderApp(
    child: LandscapePage(
      onApplyConfig: () {},
      onBack: () {},
    ),
    providers: [ChangeNotifierProvider<LandscapeModel>.value(value: model)],
  );
}

const customConf = './test/testdata/landscape/custom.conf';
const notFoundPath = './test/testdata/landscape/notfound.txt';
const caCert = './test/testdata/certs/ca_cert.pem';
const clientCert = './test/testdata/certs/client_cert.pem';
const clientKey = './test/testdata/certs/client_key.pem';
const binaryCert = './test/testdata/certs/binary_cert.der';
const notATextCert = './test/testdata/certs/not_a_cert.pem';
const notABinCert = './test/testdata/certs/not_a_cert.der';

class FakeFilePicker extends FilePicker {
  /// Fake [FilePicker] that always returns the given `paths`.
  FakeFilePicker(this.paths);

  final List<String> paths;

  @override
  Future<FilePickerResult?> pickFiles({
    String? dialogTitle,
    String? initialDirectory,
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    Function(FilePickerStatus p1)? onFileLoading,
    bool allowCompression = true,
    int compressionQuality = 30,
    bool allowMultiple = false,
    bool withData = false,
    bool withReadStream = false,
    bool lockParentWindow = false,
    bool readSequential = false,
  }) async =>
      FilePickerResult(
        paths.map((p) => PlatformFile(name: p, path: p, size: 0)).toList(),
      );
}
