import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';
import 'package:p4w_ms_store/p4w_ms_store_platform_interface.dart';
import 'package:path/path.dart' as p;
import 'package:ubuntupro/constants.dart';
import 'package:ubuntupro/core/environment.dart';
import 'package:ubuntupro/launch_agent.dart';
import 'package:ubuntupro/main.dart' as app;
import 'package:ubuntupro/pages/startup/startup_page.dart';
import 'package:ubuntupro/pages/subscription_status/subscribe_now_page.dart';
import 'package:yaru_test/yaru_test.dart';

import '../test/utils/l10n_tester.dart';
import 'utils/build_agent.dart';

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  // A temporary directory mocking the $env:LocalAppData directory to sandbox our agent.
  Directory? tmp;

  setUpAll(() async {
    await YaruTestWindow.ensureInitialized();
    // Use a random place inside the build tree as the `LOCALAPPDATA` env variable for all test cases below.
    tmp = await msixRootDir().createTemp('test-');
    Environment(
      overrides: {'LOCALAPPDATA': tmp!.path},
    );
  });

  tearDownAll(() => tmp?.delete(recursive: true));
  group('no agent build', () {
    // Verifies that a proper message is displayed when the agent cannot be run.
    testWidgets(
      'cannot run agent',
      (tester) async {
        await app.main();
        await tester.pumpAndSettle();

        final l10n = tester.l10n<StartupPage>();
        final message = find.text(l10n.agentStateCannotStart);
        expect(message, findsOneWidget);
      },
    );
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

      tearDownAll(() async {
        // kill all agent processes.
        if (Platform.isWindows) {
          await Process.run('taskkill.exe', ['/f', '/im', agentImageName]);
          // taskkill is not immediate
          await Future.delayed(const Duration(seconds: 1));
        } else {
          await Process.run(
            'killall',
            [p.basenameWithoutExtension(agentImageName)],
          );
        }
        // Finally deletes the directory.
        await Directory(agentDir).delete(recursive: true);
      });

      // Channel through which we can mock the MS Store plugin.
      const proChannel = MethodChannelP4wMsStore.methodChannel;

      tearDown(() {
        // Restores the plugin method call handler after each test, i.e. removes
        // any mocks previously installed by any test case.
        binding.defaultBinaryMessenger
            .setMockMethodCallHandler(proChannel, null);
      });

      // Tests the user journey that starts with the agent down.
      // The GUI should start the agent, check that there is no active subscription
      // and trigger a subscription purchase transaction.
      testWidgets(
        'startup to purchase',
        (tester) async {
          // For this test case the purchase operation must always succeed.
          binding.defaultBinaryMessenger.setMockMethodCallHandler(proChannel,
              (call) async {
            // The exact delay duration doesn't matter. Still good to have some delay
            // to ensure the client code won't expect things will respond instantly.
            await Future.delayed(const Duration(milliseconds: 20));
            return PurchaseStatus.succeeded.index;
          });

          await app.main();
          await tester.pumpAndSettle();

          // The "subscribe now page" is only shown if the GUI communicates with the background agent.
          final l10n = tester.l10n<SubscribeNowPage>();
          final button = find.text(l10n.subscribeNow);
          expect(button, findsOneWidget);

          await tester.tap(button);
          await tester.pumpAndSettle();

          // TODO: Update the expectation when the agent becomes able to reply the notification without crashing.
          // Most likely when the MS Store mock becomes available.
          expect(find.byType(SubscribeNowPage), findsOneWidget);
        },
      );
      testWidgets(
        'purchase failure',
        (tester) async {
          // For this test case the purchase operation must always fail.
          binding.defaultBinaryMessenger.setMockMethodCallHandler(proChannel,
              (call) async {
            // The exact delay duration doesn't matter. Still good to have some delay
            // to ensure the client code won't expect things will respond instantly.
            await Future.delayed(const Duration(milliseconds: 20));
            return PurchaseStatus.serverError.index;
          });

          await app.main();
          await tester.pumpAndSettle();

          // The "subscribe now page" is only shown if the GUI communicates with the background agent.
          final l10n = tester.l10n<SubscribeNowPage>();
          final button = find.text(l10n.subscribeNow);
          expect(button, findsOneWidget);

          await tester.tap(button);
          await tester.pumpAndSettle();

          expect(find.byType(SubscribeNowPage), findsOneWidget);
          expect(find.byType(SnackBar), findsOneWidget);
        },
      );
    },
    skip: !Platform.isWindows,
    // skips the whole group of tests if not on Windows since it relies on compiling and running the agent.
  );
}
