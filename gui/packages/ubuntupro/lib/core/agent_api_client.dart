import 'package:agentapi/agentapi.dart';
import 'package:grpc/grpc.dart';
import 'package:meta/meta.dart';

/// A type alias for the gRPC message enum which by default has a big name.
typedef SubscriptionType = SubscriptionInfo_SubscriptionType;

/// AgentApiClient hides the gRPC details in a more convenient API.
class AgentApiClient {
  AgentApiClient({required String host, required int port})
      : _client = UIClient(
          ClientChannel(
            host,
            port: port,
            options: const ChannelOptions(
              credentials: ChannelCredentials.insecure(),
            ),
          ),
        );

  final UIClient _client;

  @visibleForTesting
  AgentApiClient.withClient(this._client);

  /// Dispatches a applyProToken request with the supplied Pro [token].
  Future<void> applyProToken(String token) async {
    final info = ProAttachInfo();
    info.token = token;
    await _client.applyProToken(info);
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
