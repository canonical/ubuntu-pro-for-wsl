import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/landscape/landscape_model.dart';
import 'package:ubuntupro/pages/landscape/landscape_page.dart';
import 'package:ubuntupro/pages/widgets/page_widgets.dart';
import 'package:yaru/yaru.dart';
import 'package:yaru_test/yaru_test.dart';

import 'landscape_page_test.mocks.dart';

@GenerateMocks([AgentApiClient])
void main() {
  const tempFileName = 'Pro4WSLLandscapePageTEMP.conf';
  Directory? tempDir;
  var tempFilePath = '';

  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  // TODO: Sometimes the Column in the LandscapePage extends past the test environment's screen
  // due differences in font size between production and testing environments.
  // This should be resolved so that we don't have to specify a manual text scale factor.
  // See more: https://github.com/flutter/flutter/issues/108726#issuecomment-1205035859
  binding.platformDispatcher.textScaleFactorTestValue = 0.6;

  setUpAll(() {
    tempDir = Directory.systemTemp.createTempSync();
    tempFilePath = '${tempDir!.path}/$tempFileName';
  });

  tearDownAll(() {
    if (tempDir != null && tempDir!.existsSync()) {
      tempDir?.deleteSync(recursive: true);
    }
  });

  group('input sections', () {
    testWidgets('default state', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<FilledButton>(continueButton).enabled, isFalse);

      final radio1 = find.byWidgetPredicate(
        (widget) =>
            widget is Radio && widget.value == LandscapeConfigType.manual,
      );
      expect(radio1, findsOne);
      expect(
        tester.widget<Radio>(radio1).groupValue == LandscapeConfigType.manual,
        isTrue,
      );

      final radio2 = find.byWidgetPredicate(
        (widget) => widget is Radio && widget.value == LandscapeConfigType.file,
      );
      expect(radio2, findsOne);
      expect(
        tester.widget<Radio>(radio2).groupValue == LandscapeConfigType.manual,
        isTrue,
      );
    });

    testWidgets('continue enabled', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());
      model.fqdn = LandscapeModel.landscapeSaas;

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final continueButton = find.button(lang.buttonNext);
      expect(continueButton, findsOne);
      expect(tester.widget<FilledButton>(continueButton).enabled, isTrue);
    });
  });

  group('input', () {
    setUp(() async {
      final tempFile = File(tempFilePath);
      await tempFile.writeAsString('');
    });

    testWidgets('calls back on success manual', (tester) async {
      final agent = MockAgentApiClient();
      final model = LandscapeModel(agent);

      var applied = false;
      when(agent.applyLandscapeConfig(any)).thenAnswer((_) async {
        applied = true;
      });

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFQDNLabel),
        matching: find.byType(TextField),
      );
      final continueButton = find.button(lang.buttonNext);

      // expect false since when FQDN is landscapeSaas, an account name is required
      await tester.enterText(fqdnInput, LandscapeModel.landscapeSaas);
      await tester.pump();
      await tester.tap(continueButton);
      expect(model.accountName, isEmpty);
      expect(applied, isFalse);

      // an account name is now provided, so we expect applied
      final accountNameInput = find.ancestor(
        of: find.text(lang.landscapeAccountNameLabel),
        matching: find.byType(TextField),
      );
      await tester.enterText(accountNameInput, 'test');
      await tester.pump();
      await tester.tap(continueButton);
      await tester.pump();
      expect(model.accountName, 'test');
      expect(applied, isTrue);
    });

    testWidgets('calls back on success file', (tester) async {
      final agent = MockAgentApiClient();
      final model = LandscapeModel(MockAgentApiClient());

      var applied = false;
      when(agent.applyLandscapeConfig(any)).thenAnswer((_) async {
        applied = true;
      });

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fileInput = find.ancestor(
        of: find.text(lang.landscapeFileLabel),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);
      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, tempFilePath);
      await tester.pump();

      final continueButton = find.button(lang.buttonNext);
      expect(tester.widget<FilledButton>(continueButton).enabled, isTrue);
      expect(applied, isFalse);

      // Ideally we'd test if `applied` is true after continue is pressed,
      // however Flutter has issues with reading files asynchronously in this
      // test environment
    });

    testWidgets('feedback on manual error', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fqdnInput = find.ancestor(
        of: find.text(lang.landscapeFQDNLabel),
        matching: find.byType(TextField),
      );
      expect(fqdnInput, findsOne);

      await tester.enterText(fqdnInput, '::');
      await tester.pump();

      final errorText = find.text(lang.landscapeFQDNError);
      expect(errorText, findsOne);
    });

    testWidgets('feedback on file error', (tester) async {
      final model = LandscapeModel(MockAgentApiClient());

      final app = buildApp(model);
      await tester.pumpWidget(app);
      final context = tester.element(find.byType(ColumnLandingPage));
      final lang = AppLocalizations.of(context);

      final fileInput = find.ancestor(
        of: find.text(lang.landscapeFileLabel),
        matching: find.byType(TextField),
      );
      expect(fileInput, findsOne);
      await tester.tap(fileInput);
      await tester.pump();

      await tester.enterText(fileInput, '${tempDir!.path}/Invalid.conf');
      await tester.pump();

      final errorText = find.text(lang.landscapeFileNotFound);
      expect(errorText, findsOne);
    });
  });

  testWidgets('creates a model', (tester) async {
    final mockClient = MockAgentApiClient();
    registerServiceInstance<AgentApiClient>(mockClient);
    const app = MaterialApp(
      routes: {'/': LandscapePage.create},
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    );

    await tester.pumpWidget(app);
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(LandscapePage));
    final model = Provider.of<LandscapeModel>(context, listen: false);

    expect(model, isNotNull);
  });
}

Widget buildApp(
  LandscapeModel model,
) {
  return YaruTheme(
    builder: (context, yaru, child) => MaterialApp(
      theme: yaru.theme,
      darkTheme: yaru.darkTheme,
      home: Scaffold(
        body: ChangeNotifierProvider<LandscapeModel>(
          create: (_) => model,
          child: LandscapePage(
            onApplyConfig: () {},
          ),
        ),
      ),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    ),
  );
}
