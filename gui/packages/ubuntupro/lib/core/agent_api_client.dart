import 'package:agentapi/agentapi.dart';
import 'package:grpc/grpc.dart';
import 'google/protobuf/empty.pb.dart' as $emptypb;

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

  /// Dispatches a applyProToken request with the supplied Pro [token].
  Future<void> applyProToken(String token) async {
    final info = ProAttachInfo();
    info.token = token;
    await _client.applyProToken(info);
  }

  /// Attempts to ping the Agent Service at the supplied endpoint
  /// ([host] and [port]). Returns true on success.
  Future<bool> ping() => _client
      .ping($emptypb.Empty())
      .then((_) => true)
      .onError<GrpcError>((_, __) => false);
}
