import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:p4w_ms_store/p4w_ms_store_method_channel.dart';

void main() {
  final messenger =
      TestWidgetsFlutterBinding.ensureInitialized().defaultBinaryMessenger;
  final platform = MethodChannelP4wMsStore();
  final channel = platform.methodChannel;

  setUp(() {
    // Overrides the binary messenger method call handler with a stub that
    // pretends to succeed but does nothing.
    messenger.setMockMethodCallHandler(
      channel,
      (methodCall) async {
        return;
      },
    );
  });

  tearDown(() {
    // Resets the binary messenger method call handler.
    messenger.setMockMethodCallHandler(channel, null);
  });

  test('launchFullTrustProcess completes', () async {
    expect(platform.launchFullTrustProcess(), completes);
  });

  test('launchFullTrustProcess fails', () async {
    // Overrides the binary messenger method call handler with a stub that
    // mimics a launch failure.
    messenger.setMockMethodCallHandler(
      channel,
      (methodCall) async {
        throw PlatformException(code: 'test');
      },
    );

    expect(
      platform.launchFullTrustProcess(),
      throwsA(isA<PlatformException>()),
    );
  });
}
