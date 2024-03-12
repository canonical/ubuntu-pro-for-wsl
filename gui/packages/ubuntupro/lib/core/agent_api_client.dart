import 'package:agentapi/agentapi.dart';
import 'package:grpc/grpc.dart';
import 'package:meta/meta.dart';

/// Type aliases for the gRPC message enums which by default have big names.
typedef SubscriptionType = SubscriptionInfo_SubscriptionType;
typedef LandscapeSourceType = LandscapeSource_LandscapeSourceType;

/// AgentApiClient hides the gRPC details in a more convenient API.
class AgentApiClient {
  AgentApiClient({
    required String host,
    required int port,
    this.stubFactory = UIClient.new,
  }) : _channel = ClientChannel(
          host,
          port: port,
          options: const ChannelOptions(
            credentials: ChannelCredentials.insecure(),
          ),
        ) {
    _client = stubFactory.call(_channel);
  }

  /// A factory for UIClient and derived classes objects, only meaningful for testing.
  /// In production it should always default to [UIClient.new].
  @visibleForTesting
  final UIClient Function(ClientChannel) stubFactory;

  /// Never null, but reassignable inside [connectTo].
  late UIClient _client;
  ClientChannel _channel;

  /// Changes the endpoint this API client is connected to.
  Future<bool> connectTo({required String host, required int port}) {
    _channel.shutdown();
    _channel = ClientChannel(
      host,
      port: port,
      options: const ChannelOptions(
        credentials: ChannelCredentials.insecure(),
      ),
    );
    _client = stubFactory.call(_channel);
    return ping();
  }

  /// Dispatches a applyProToken request with the supplied Pro [token].
  Future<SubscriptionInfo> applyProToken(String token) {
    final info = ProAttachInfo();
    info.token = token;
    return _client.applyProToken(info);
  }

  Future<LandscapeSource> applyLandscapeConfig(String config) {
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

  /// Returns current information about the config sources, if any, to determine which parts of the UI are enabled.
  Future<ConfigSources> configSources() => _client.getConfigSources(Empty());

  /// Notifies the background agent of a succesfull purchase transaction on MS Store.
  /// It's expected that an updated SubscriptionInfo will be returned.
  Future<SubscriptionInfo> notifyPurchase() => _client.notifyPurchase(Empty());

  Stream<ConnectionEvent> get onConnectionChanged =>
      mapGRPCConnectionEvents(_channel.onConnectionStateChanged);
}

enum ConnectionEvent {
  dropped,
  connected,
}

/// Maps gRPC connection events to a stream of [ConnectionEvent] enum values.
Stream<ConnectionEvent> mapGRPCConnectionEvents(
  Stream<ConnectionState> stream,
) {
  return stream.map((event) {
    // Being in idle is not a problem for gRPC. It quickly reconnects and
    // dispatches an RPC call successfully if the server is still up.
    if (event == ConnectionState.ready || event == ConnectionState.idle) {
      return ConnectionEvent.connected;
    }

    return ConnectionEvent.dropped;
  });
}
