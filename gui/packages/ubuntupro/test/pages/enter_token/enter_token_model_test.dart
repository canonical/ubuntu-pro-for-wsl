import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:ubuntupro/core/agent_api_client.dart';
import 'package:ubuntupro/core/pro_token.dart';
import 'package:ubuntupro/pages/enter_token/enter_token_model.dart';

import 'enter_token_model_test.mocks.dart';
import 'token_samples.dart' as tks;

@GenerateMocks([AgentApiClient])
void main() {
  test('Model errors', () async {
    final model = EnterProTokenModel(MockAgentApiClient());

    model.update('');

    expect(model.errorOrNull, TokenError.empty);

    model.update(tks.tooShort);

    expect(model.errorOrNull, TokenError.tooShort);

    model.update(tks.tooLong);

    expect(model.errorOrNull, TokenError.tooLong);

    model.update(tks.invalidPrefix);

    expect(model.errorOrNull, TokenError.invalidPrefix);

    model.update(tks.invalidEncoding);

    expect(model.errorOrNull, TokenError.invalidEncoding);
  });
  test('accessors on success', () {
    final model = EnterProTokenModel(MockAgentApiClient());
    final tokenInstance = ProToken.create(tks.good).orNull();

    model.update(tks.good);

    expect(model.hasError, isFalse);
    expect(model.errorOrNull, isNull);
    expect(model.token, tks.good);
    expect(model.valueOrNull!.value, tks.good);
    expect(model.valueOrNull, tokenInstance);
    expect(model.value, equals(ProToken.create(tks.good)));
  });

  test('notify listeners', () {
    final model = EnterProTokenModel(MockAgentApiClient());
    var notified = false;
    model.addListener(() {
      notified = true;
    });

    model.update(tks.good);

    expect(notified, isTrue);
  });

  test('apply only when no errors', () {
    final mock = MockAgentApiClient();
    final model = EnterProTokenModel(mock);

    model.update(tks.tooLong);
    model.apply();

    expect(model.hasError, isTrue);
    verifyNever(mock.proAttach(tks.tooLong));

    model.update(tks.good);
    model.apply();

    expect(model.hasError, isFalse);
    verify(mock.proAttach(tks.good)).called(1);
  });
}
