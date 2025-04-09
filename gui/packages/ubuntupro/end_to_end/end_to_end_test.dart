import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:stack_trace/stack_trace.dart' as stack_trace;
import 'package:ubuntupro/core/environment.dart';
import 'package:ubuntupro/main.dart' as app;
import 'package:ubuntupro/pages/subscribe_now/subscribe_now_page.dart';
import 'package:ubuntupro/pages/subscription_status/subscription_status_page.dart';

import '../test/utils/l10n_tester.dart';

const proTokenEnv = 'UP4W_TEST_PRO_TOKEN';

typedef TestCases = Map<String, Future<void> Function(WidgetTester)>;

void main(List<String> args) {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  FlutterError.demangleStackTrace = (stack) {
    if (stack is stack_trace.Trace) return stack.vmTrace;
    if (stack is stack_trace.Chain) return stack.toTrace().vmTrace;
    return stack;
  };

  const testCases = {
    'TestOrganizationProvidedToken': testOrganizationProvidedToken,
    'TestCloudInitIntegration': testOrganizationProvidedToken,
    'TestManualTokenInput': testManualTokenInput,
    'TestPurchase': testPurchase,
  };

  final scenario = args[0];

  if (!testCases.keys.contains(scenario)) {
    debugPrint('"$scenario" is not a valid test scenario.');
    exit(1);
  }

  // For a single run of the end-to-end tests we can only have one active GUI test scenario,
  // what implies that we must define a single 'testWidgets'. The actual function body will be determined at runtime.
  testWidgets(scenario, testCases[scenario]!, skip: !Platform.isWindows);
}

Future<void> testOrganizationProvidedToken(WidgetTester tester) async {
  await app.main();
  await tester.pumpAndSettle();

  // asserts that we transitioned to the organization-managed status page.
  final l10n = tester.l10n<SubscriptionStatusPage>();
  expect(find.text(l10n.manageUbuntuPro), findsNothing);
}

Future<void> testManualTokenInput(WidgetTester tester) async {
  await app.main();
  await tester.pumpAndSettle();

  // The "subscribe now page" is only shown if the GUI communicates with the background agent.
  var l10n = tester.l10n<SubscribeNowPage>();

  // finds the pro token from the environment
  final goodToken = Environment()[proTokenEnv];
  expect(
    goodToken,
    isNotNull,
    reason: '$proTokenEnv must be set to a valid token.',
  );

  // enters a good token value
  final inputField = find.byType(TextField);
  await tester.enterText(inputField, goodToken!);
  await tester.pumpAndSettle();

  // submits the input.
  final button = find.text(l10n.attach);
  await tester.tap(button);
  await tester.pumpAndSettle();

  // asserts that we transitioned to the user-managed status page.
  l10n = tester.l10n<SubscriptionStatusPage>();
  expect(find.text(l10n.detachPro), findsOneWidget);
}

Future<void> testPurchase(WidgetTester tester) async {
  await app.main();
  await tester.pumpAndSettle();

  // The "subscribe now page" is only shown if the GUI communicates with the background agent.
  var l10n = tester.l10n<SubscribeNowPage>();
  final button = find.text(l10n.getUbuntuPro);
  expect(button, findsOneWidget);

  await tester.tap(button);
  await tester.pumpAndSettle();

  // asserts that we transitioned to the store-managed status page.
  l10n = tester.l10n<SubscriptionStatusPage>();
  expect(find.text(l10n.manageUbuntuPro), findsOneWidget);
}
