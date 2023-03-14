import 'package:agentapi/agentapi.dart';
import 'package:grpc/grpc.dart';

/// AgentApiClient hides the gRPC details in a more convenient API.
class AgentApiClient {
  AgentApiClient({required this.host, required this.port})
      : _client = UIClient(
          ClientChannel(
            host,
            port: port,
            options: const ChannelOptions(
              credentials: ChannelCredentials.insecure(),
            ),
          ),
        );

  /// The Agent gRPC Service host address.
  final String host;

  /// The Agent gRPC Service port.
  final int port;

  final UIClient _client;

  /// Dispatches a ProAttach request with the supplied Pro [token].
  Future<void> proAttach(String token) async =>
      await _client.proAttach(AttachInfo(token: token));

  /// Attempts to ping the Agent Service at the supplied endpoint
  /// ([host] and [port]). Returns true on success.
  Future<bool> ping() => _client
      .ping(Empty())
      .then((_) => true)
      .onError<GrpcError>((_, __) => false);
}
