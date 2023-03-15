import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/mockito.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/pages/enter_token/enter_token_model.dart';
import 'package:ubuntupro/pages/enter_token/enter_token_page.dart';

import 'enter_token_model_test.mocks.dart';
import 'token_samples.dart';

MaterialApp buildApp(MockAgentApiClient mock) => MaterialApp(
      home: ChangeNotifierProvider(
        create: (_) => EnterProTokenModel(mock),
        child: const EnterProTokenPage(title: 'p4W'),
      ),
    );

void main() {
  group('basic flow', () {
    final app = buildApp(MockAgentApiClient());

    testWidgets('starts with no error', (tester) async {
      await tester.pumpWidget(app);

      final input = tester.firstWidget<TextField>(find.byType(TextField));

      expect(input.decoration!.errorText, isNull);

      final button =
          tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
      expect(button.enabled, isTrue);
    });

    testWidgets('too short', (tester) async {
      await tester.pumpWidget(app);
      final textFieldFinder = find.byType(TextField);

      await tester.enterText(textFieldFinder, 'Blah');
      await tester.pump();

      final input = tester.firstWidget<TextField>(textFieldFinder);
      expect(input.decoration!.errorText, isNotNull);
      expect(input.decoration!.errorText, contains('too short'));

      final button =
          tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
      expect(button.enabled, isFalse);
    });

    testWidgets('too long', (tester) async {
      await tester.pumpWidget(app);
      final textFieldFinder = find.byType(TextField);

      await tester.enterText(
        textFieldFinder,
        'BlahBlahBlahBlahBlahBlahBlahBlah',
      );
      await tester.pump();

      final input = tester.firstWidget<TextField>(textFieldFinder);
      expect(input.decoration!.errorText, isNotNull);
      expect(input.decoration!.errorText, contains('too long'));

      final button =
          tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
      expect(button.enabled, isFalse);
    });

    testWidgets('good token', (tester) async {
      await tester.pumpWidget(app);
      final textFieldFinder = find.byType(TextField);

      await tester.enterText(textFieldFinder, good);
      await tester.pump();

      final input = tester.firstWidget<TextField>(textFieldFinder);
      expect(input.decoration!.errorText, isNull);

      final button =
          tester.firstWidget<ElevatedButton>(find.byType(ElevatedButton));
      expect(button.enabled, isTrue);
    });
  });

  testWidgets('calls proAttach', (tester) async {
    final mock = MockAgentApiClient();
    final app = buildApp(mock);
    await tester.pumpWidget(app);
    final textFieldFinder = find.byType(TextField);

    await tester.enterText(
      textFieldFinder,
      good,
    );
    await tester.pump();

    await tester.tap(find.byType(ElevatedButton));
    verify(mock.proAttach(good)).called(1);
  });

  testWidgets('creates a model', (tester) async {
    final mock = MockAgentApiClient();
    registerServiceInstance<AgentApiClient>(mock);
    await tester.pumpWidget(
      const MaterialApp(
        routes: {'/': EnterProTokenPage.create},
      ),
    );

    final page = find.byType(EnterProTokenPage);
    expect(page, findsOneWidget);

    final context = tester.element(page);
    final model = Provider.of<EnterProTokenModel>(context, listen: false);
    expect(model, isNotNull);
  });
}
