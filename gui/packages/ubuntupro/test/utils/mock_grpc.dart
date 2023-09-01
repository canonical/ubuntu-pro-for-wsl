// A simple mock of the gRPC ResponseFuture class that allows predefined response values.
import 'package:agentapi/agentapi.dart';
import 'package:async/async.dart';
import 'package:grpc/grpc.dart';

class MockedResponse<R> extends DelegatingFuture<R>
    implements ResponseFuture<R> {
  /// The constructor accepts the [value] that the future will return when awaited on.
  MockedResponse(R value) : super(Future.value(value));

  // Placeholders to fullfill the ResponseFuture<R> API.
  @override
  Future<void> cancel() {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String>> get headers => throw UnimplementedError();

  @override
  Future<Map<String, String>> get trailers => throw UnimplementedError();
}

/// A stateful mock of the UIClient gRPC service.
class MockUIClient extends UIClient {
  SubscriptionInfo subscriptionInfo = SubscriptionInfo();

  MockUIClient()
      : super(
          ClientChannel(
            '127.0.0.1',
            port: 9,
            options: const ChannelOptions(
              credentials: ChannelCredentials.insecure(),
            ),
          ),
        ) {
    subscriptionInfo.ensureNone();
  }

  @override
  ResponseFuture<Empty> applyProToken(
    ProAttachInfo request, {
    CallOptions? options,
  }) {
    if (request.token.isEmpty) {
      subscriptionInfo.ensureNone();
    } else {
      subscriptionInfo.ensureUser();
    }
    return MockedResponse(Empty());
  }

  @override
  ResponseFuture<SubscriptionInfo> getSubscriptionInfo(
    Empty request, {
    CallOptions? options,
  }) {
    return MockedResponse(subscriptionInfo);
  }

  @override
  ResponseFuture<Empty> ping(Empty request, {CallOptions? options}) {
    return MockedResponse(Empty());
  }
}
