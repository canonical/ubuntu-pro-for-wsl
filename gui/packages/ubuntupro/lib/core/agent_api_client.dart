import 'package:agentapi/agentapi.dart';
import 'package:grpc/grpc.dart';
import 'package:meta/meta.dart';

/// A type alias for the gRPC message enum which by default has a big name.
typedef SubscriptionType = SubscriptionInfo_SubscriptionType;

/// AgentApiClient hides the gRPC details in a more convenient API.
class AgentApiClient {
  AgentApiClient({
    required String host,
    required int port,
    this.stubFactory = UIClient.new,
  }) : _client = stubFactory.call(
          ClientChannel(
            host,
            port: port,
            options: const ChannelOptions(
              credentials: ChannelCredentials.insecure(),
            ),
          ),
        );

  /// A factory for UIClient and derived classes objects, only meaningful for testing.
  /// In production it should always default to [UIClient.new].
  @visibleForTesting
  final UIClient Function(ClientChannel) stubFactory;

  /// Never null, but reassignable inside [connectTo].
  late UIClient _client;

  /// Changes the endpoint this API client is connected to.
  Future<bool> connectTo({required String host, required int port}) {
    _client = stubFactory.call(ClientChannel(
      host,
      port: port,
      options: const ChannelOptions(
        credentials: ChannelCredentials.insecure(),
      ),
    ));
    return ping();
  }

  /// Dispatches a applyProToken request with the supplied Pro [token].
  Future<SubscriptionInfo> applyProToken(String token) {
    final info = ProAttachInfo();
    info.token = token;
    return _client.applyProToken(info);
  }

  Future<void> applyLandscapeConfig(String config) {
    final request = LandscapeConfig();
    request.config = config;
    return _client.applyLandscapeConfig(request);
  }

  /// Attempts to ping the Agent Service at the supplied endpoint
  /// ([host] and [port]). Returns true on success.
  Future<bool> ping() => _client
      .ping(Empty())
      .then((_) => true)
      .onError<GrpcError>((_, __) => false);

  /// Returns information about the current subscription, if any.
  Future<SubscriptionInfo> subscriptionInfo() =>
      _client.getSubscriptionInfo(Empty());

  /// Notifies the background agent of a succesfull purchase transaction on MS Store.
  /// It's expected that an updated SubscriptionInfo will be returned.
  Future<SubscriptionInfo> notifyPurchase() => _client.notifyPurchase(Empty());
}
