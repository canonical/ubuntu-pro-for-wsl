import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';
import 'package:path/path.dart' as p;
import 'package:stack_trace/stack_trace.dart' as stack_trace;
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/constants.dart';
import 'package:ubuntupro/core/environment.dart';
import 'package:ubuntupro/launch_agent.dart';
import 'package:ubuntupro/main.dart' as app;
import 'package:ubuntupro/pages/landscape/landscape_page.dart';
import 'package:ubuntupro/pages/landscape_skip/landscape_skip_page.dart';
import 'package:ubuntupro/pages/startup/startup_page.dart';
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_page.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_page.dart';
import 'package:yaru/widgets.dart';
import 'package:yaru_test/yaru_test.dart';

import '../test/utils/l10n_tester.dart';
import 'utils/build_agent.dart';

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  FlutterError.demangleStackTrace = (stack) {
    if (stack is stack_trace.Trace) return stack.vmTrace;
    if (stack is stack_trace.Chain) return stack.toTrace().vmTrace;
    return stack;
  };

  // A temporary directory mocking the $env:UserProfile directory to sandbox our agent.
  Directory? tmpHome;
  Directory? tmpLocalAppData;

  setUpAll(() async {
    await YaruTestWindow.ensureInitialized();
    // Use a random place inside the build tree as the `USERPROFILE` env variable for all test cases below.
    tmpHome = await msixRootDir().createTemp('test-');

    tmpLocalAppData = Directory(p.join(tmpHome!.path, 'AppData/Local'));
    await tmpLocalAppData!.create(recursive: true);

    Environment(
      overrides: {
        'USERPROFILE': tmpHome!.path,
        'LOCALAPPDATA': tmpLocalAppData!.path,
        'UP4W_INTEGRATION_TESTING': 'true',
      },
    );
  });

  tearDownAll(() => tmpHome?.delete(recursive: true));
  group('no agent build', () {
    // Verifies that a proper message is displayed when the agent cannot be run.
    testWidgets('cannot run agent', (tester) async {
      await app.main();
      await tester.pumpAndSettle();

      final l10n = tester.l10n<StartupPage>();
      final message = find.text(l10n.agentStateCannotStart);
      expect(message, findsOneWidget);
    });
  });

  group(
    'build the agent',
    () {
      // finds the directory where the agent executable should be placed.
      final agentFullPath = p.join(msixRootDir().path, kAgentRelativePath);
      final agentDir = p.dirname(agentFullPath);
      final agentImageName = p.basename(agentFullPath);

      setUpAll(() async {
        await buildAgentExe(agentDir);
      });

      tearDown(() async {
        // kill all agent processes.
        if (Platform.isWindows) {
          await Process.run('taskkill.exe', ['/f', '/im', agentImageName]);
          // taskkill is not immediate
          await Future.delayed(const Duration(seconds: 1));
        } else {
          await Process.run('killall', [
            p.basenameWithoutExtension(agentImageName),
          ]);
        }
        File(p.join(tmpHome!.path, '.ubuntupro/.address')).deleteSync();
      });

      tearDownAll(() async {
        // Finally deletes the directory.
        await Directory(agentDir).delete(recursive: true);
      });

      // Channel through which we can mock the MS Store plugin.
      const proChannel = MethodChannelP4wMsStore.methodChannel;

      tearDown(() {
        // Restores the plugin method call handler after each test, i.e. removes
        // any mocks previously installed by any test case.
        binding.defaultBinaryMessenger.setMockMethodCallHandler(
          proChannel,
          null,
        );
        resetAllServices();
      });

      // Tests the user journey for manual input of a pro token followed by a detach call.
      testWidgets('manual skip Landscape', (tester) async {
        await app.main();
        await tester.pumpAndSettle();

        // The "subscribe now page" is only shown if the GUI communicates with the background agent.
        var l10n = tester.l10n<SubscribeNowPage>();

        // enters a good token value
        final inputField = find.byType(TextField);
        await tester.enterText(inputField, 'CJd8MMN8wXSWsv7wJT8c8dDK');
        await tester.pump();

        // submits the input.
        final button = find.text(l10n.attach);
        await tester.tap(button);
        await tester.pumpAndSettle();

        // check we transition to skip page
        l10n = tester.l10n<LandscapeSkipPage>();
        final radios = find.byType(YaruSelectableContainer);
        expect(radios, findsNWidgets(2));
        final skip = find.text(l10n.landscapeSkip);
        await tester.tap(skip);
        await tester.pump();
        final next = find.button(l10n.buttonNext);
        await tester.tap(next);
        await tester.pumpAndSettle();

        // checks that we transitioned to the SubscriptionStatusPage
        l10n = tester.l10n<SubscriptionStatusPage>();

        // finds and taps the "detach pro" button.
        final detachButton = find.text(l10n.detachPro);
        expect(detachButton, findsOneWidget);
        await tester.tap(detachButton);
        await tester.pumpAndSettle();

        // checks that we went back to the SubscribeNowPage
        expect(find.byType(SubscribeNowPage), findsOneWidget);
      });

      testWidgets('manual with landscape', (tester) async {
        await app.main();
        await tester.pumpAndSettle();

        // The "subscribe now page" is only shown if the GUI communicates with the background agent.
        var l10n = tester.l10n<SubscribeNowPage>();

        // enters a good token value
        final inputField = find.byType(TextField);
        await tester.enterText(inputField, 'CJd8MMN8wXSWsv7wJT8c8dDK');
        await tester.pump();

        // submits the input.
        final button = find.text(l10n.attach);
        await tester.tap(button);
        await tester.pumpAndSettle();

        // check we transition to skip page
        l10n = tester.l10n<LandscapeSkipPage>();
        final radios = find.byType(YaruSelectableContainer);
        expect(radios, findsNWidgets(2));
        final register = find.text(l10n.landscapeSkipRegister);
        await tester.tap(register);
        await tester.pump();
        final next = find.button(l10n.buttonNext);
        await tester.tap(next);
        await tester.pumpAndSettle();

        // check that we transitioned to the LandscapePage
        l10n = tester.l10n<LandscapePage>();
        final continueButton = find.button(l10n.landscapeRegister);

        // check that invalid input disables continue
        await tester.tap(find.text(l10n.landscapeSetupManual));
        await tester.pump();
        final fqdnInput = find.ancestor(
          of: find.text(l10n.landscapeFQDNLabel),
          matching: find.byType(TextField),
        );
        await tester.enterText(fqdnInput, ':::');
        await tester.pump();
        expect(
          tester.widget<ButtonStyleButton>(continueButton).enabled,
          isFalse,
        );

        // check that valid input enabled continue, and continue
        await tester.enterText(fqdnInput, 'localhost');
        await tester.pump();
        expect(
          tester.widget<ButtonStyleButton>(continueButton).enabled,
          isTrue,
        );
        await tester.tap(continueButton);
        await tester.pumpAndSettle();

        // checks that we transitioned to the SubscriptionStatusPage
        l10n = tester.l10n<SubscriptionStatusPage>();

        // finds and taps the "detach pro" button.
        final detachButton = find.text(l10n.detachPro);
        expect(detachButton, findsOneWidget);
        await tester.tap(detachButton);
        await tester.pumpAndSettle();

        // checks that we went back to the SubscribeNowPage
        expect(find.byType(SubscribeNowPage), findsOneWidget);
      });

      // Tests the user journey that starts with the agent down.
      // The GUI should start the agent, check that there is no active subscription
      // and trigger a subscription purchase transaction.
      testWidgets('startup to purchase', (tester) async {
        // For this test case the purchase operation must always succeed.
        binding.defaultBinaryMessenger.setMockMethodCallHandler(proChannel, (
          call,
        ) async {
          // The exact delay duration doesn't matter. Still good to have some delay
          // to ensure the client code won't expect things will respond instantly.
          await Future.delayed(const Duration(milliseconds: 20));
          return PurchaseStatus.succeeded.index;
        });

        await app.main();
        await tester.pumpAndSettle();

        // The "subscribe now page" is only shown if the GUI communicates with the background agent.
        final l10n = tester.l10n<SubscribeNowPage>();
        final button = find.text(l10n.getUbuntuPro);
        expect(button, findsOneWidget);

        await tester.tap(button);
        await tester.pumpAndSettle();

        // TODO: Update the expectation when the agent becomes able to reply the notification without crashing.
        // Most likely when the MS Store mock becomes available.
        expect(find.byType(SubscribeNowPage), findsOneWidget);
      });
      testWidgets('purchase failure', (tester) async {
        // For this test case the purchase operation must always fail.
        binding.defaultBinaryMessenger.setMockMethodCallHandler(proChannel, (
          call,
        ) async {
          await Future.delayed(const Duration(milliseconds: 20));
          return PurchaseStatus.serverError.index;
        });

        await app.main();
        await tester.pumpAndSettle();

        // The "subscribe now page" is only shown if the GUI communicates with the background agent.
        final l10n = tester.l10n<SubscribeNowPage>();
        final button = find.text(l10n.getUbuntuPro);
        expect(button, findsOneWidget);

        await tester.tap(button);
        await tester.pumpAndSettle();

        expect(find.byType(SubscribeNowPage), findsOneWidget);
        expect(find.byType(SnackBar), findsOneWidget);
      });
      // Tests the journey in which the user is already subscribed.
      group('already subscribed', () {
        // config created by the agent.
        const configContents = '''
subscription:
    user: CJd8MMN8wXSWsv7wJT8c8dDK
    store: ""
    checksum: ""
landscape:
    config: |
        [host]
        url = landscape.canonical.com:6554
        [client]
        account_name = test-user
        registration_key =
        url = https://landscape.canonical.com/message-system
        log_level = debug
        ping_url = https://landscape.canonical.com/ping
    uid: b95842dd41454d29970040fe9492bc88
    checksum: jE8mRRqhDcoheaansWXCtjicU9L3vupxaxwk9JKBmLy+xJLaH+9WPWTmJrBMxBLI7N6gVDzpMEU3RhCyIVYDLA==
''';
        Directory? configDir;
        setUpAll(() async {
          // seed a pre-existing configuration.
          configDir = await Directory(
            p.join(tmpLocalAppData!.path, 'Ubuntu Pro'),
          ).create(recursive: true);
          final config = File(p.join(configDir!.path, 'config'));
          await config.writeAsString(configContents);
        });

        tearDownAll(() {
          configDir?.deleteSync(recursive: true);
        });

        testWidgets('success', (tester) async {
          await app.main();
          await tester.pumpAndSettle();

          // We should have transitioned straight into the Subscription status page.
          final l10n = tester.l10n<SubscriptionStatusPage>();

          // Reconfigure Landscape
          final landscapeButton = find.text(l10n.landscapeConfigureButton);
          expect(landscapeButton, findsOneWidget);
          await tester.tap(landscapeButton);
          await tester.pumpAndSettle();
          final landscapeL10n = tester.l10n<LandscapePage>();
          await tester.tap(find.text(l10n.landscapeSetupManual));
          final fqdnInput = find.ancestor(
            of: find.text(landscapeL10n.landscapeFQDNLabel),
            matching: find.byType(TextField),
          );
          await tester.tap(fqdnInput);
          await tester.enterText(fqdnInput, 'localhost');
          await tester.pump();
          final continueButton = find.button(landscapeL10n.landscapeRegister);
          await tester.tap(continueButton);
          await tester.pumpAndSettle();

          final config = File(p.join(configDir!.path, 'config'));
          final c = await config.readAsString();
          expect(c, contains('localhost'));
          expect(c, isNot(contains('landscape.canonical.com')));

          // finds and taps the "detach pro" button.
          final detachButton = find.text(l10n.detachPro);
          expect(detachButton, findsOneWidget);
          await tester.tap(detachButton);
          await tester.pumpAndSettle();

          // checks that we went back to the SubscribeNowPage
          expect(find.byType(SubscribeNowPage), findsOneWidget);
        });
      });
    },
    skip: !Platform.isWindows,
    // skips the whole group of tests if not on Windows since it relies on compiling and running the agent.
  );
}
