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
  SubscriptionInfo subs =
      SubscriptionInfo()
        ..ensureNone()
        ..freeze();

  LandscapeSource landscape =
      LandscapeSource()
        ..ensureNone()
        ..freeze();

  MockUIClient(super.channel);

  @override
  ResponseFuture<LandscapeSource> applyLandscapeConfig(
    LandscapeConfig request, {
    CallOptions? options,
  }) {
    if (request.config.isEmpty) {
      landscape = landscape.rebuild((ls) {
        ls.ensureNone();
      });
    } else {
      landscape = landscape.rebuild((ls) {
        ls.ensureUser();
      });
    }
    return MockedResponse(landscape);
  }

  @override
  ResponseFuture<SubscriptionInfo> applyProToken(
    ProAttachInfo request, {
    CallOptions? options,
  }) {
    if (request.token.isEmpty) {
      subs = subs.rebuild((s) {
        s.ensureNone();
      });
    } else {
      subs = subs.rebuild((s) {
        s.ensureUser();
      });
    }
    return MockedResponse(subs);
  }

  @override
  ResponseFuture<ConfigSources> getConfigSources(
    Empty request, {
    CallOptions? options,
  }) {
    return MockedResponse(
      ConfigSources(landscapeSource: landscape, proSubscription: subs),
    );
  }

  @override
  ResponseFuture<Empty> ping(Empty request, {CallOptions? options}) {
    return MockedResponse(Empty());
  }
}
