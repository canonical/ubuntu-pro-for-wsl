//
//  Generated code. Do not modify.
//  source: agentapi.proto
//
// @dart = 2.12

// ignore_for_file: annotate_overrides, camel_case_types
// ignore_for_file: constant_identifier_names, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_final_fields
// ignore_for_file: unnecessary_import, unnecessary_this, unused_import

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'agentapi.pb.dart' as $0;

export 'agentapi.pb.dart';

@$pb.GrpcServiceName('agentapi.UI')
class UIClient extends $grpc.Client {
  static final _$applyProToken = $grpc.ClientMethod<$0.ProAttachInfo, $0.Empty>(
      '/agentapi.UI/ApplyProToken',
      ($0.ProAttachInfo value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));
  static final _$ping = $grpc.ClientMethod<$0.Empty, $0.Empty>(
      '/agentapi.UI/Ping',
      ($0.Empty value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));
  static final _$getSubscriptionInfo = $grpc.ClientMethod<$0.Empty, $0.SubscriptionInfo>(
      '/agentapi.UI/GetSubscriptionInfo',
      ($0.Empty value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.SubscriptionInfo.fromBuffer(value));

  UIClient($grpc.ClientChannel channel,
      {$grpc.CallOptions? options,
      $core.Iterable<$grpc.ClientInterceptor>? interceptors})
      : super(channel, options: options,
        interceptors: interceptors);

  $grpc.ResponseFuture<$0.Empty> applyProToken($0.ProAttachInfo request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$applyProToken, request, options: options);
  }

  $grpc.ResponseFuture<$0.Empty> ping($0.Empty request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$ping, request, options: options);
  }

  $grpc.ResponseFuture<$0.SubscriptionInfo> getSubscriptionInfo($0.Empty request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$getSubscriptionInfo, request, options: options);
  }
}

@$pb.GrpcServiceName('agentapi.UI')
abstract class UIServiceBase extends $grpc.Service {
  $core.String get $name => 'agentapi.UI';

  UIServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ProAttachInfo, $0.Empty>(
        'ApplyProToken',
        applyProToken_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ProAttachInfo.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Empty, $0.Empty>(
        'Ping',
        ping_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Empty.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Empty, $0.SubscriptionInfo>(
        'GetSubscriptionInfo',
        getSubscriptionInfo_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Empty.fromBuffer(value),
        ($0.SubscriptionInfo value) => value.writeToBuffer()));
  }

  $async.Future<$0.Empty> applyProToken_Pre($grpc.ServiceCall call, $async.Future<$0.ProAttachInfo> request) async {
    return applyProToken(call, await request);
  }

  $async.Future<$0.Empty> ping_Pre($grpc.ServiceCall call, $async.Future<$0.Empty> request) async {
    return ping(call, await request);
  }

  $async.Future<$0.SubscriptionInfo> getSubscriptionInfo_Pre($grpc.ServiceCall call, $async.Future<$0.Empty> request) async {
    return getSubscriptionInfo(call, await request);
  }

  $async.Future<$0.Empty> applyProToken($grpc.ServiceCall call, $0.ProAttachInfo request);
  $async.Future<$0.Empty> ping($grpc.ServiceCall call, $0.Empty request);
  $async.Future<$0.SubscriptionInfo> getSubscriptionInfo($grpc.ServiceCall call, $0.Empty request);
}
@$pb.GrpcServiceName('agentapi.WSLInstance')
class WSLInstanceClient extends $grpc.Client {
  static final _$connected = $grpc.ClientMethod<$0.DistroInfo, $0.Port>(
      '/agentapi.WSLInstance/Connected',
      ($0.DistroInfo value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Port.fromBuffer(value));

  WSLInstanceClient($grpc.ClientChannel channel,
      {$grpc.CallOptions? options,
      $core.Iterable<$grpc.ClientInterceptor>? interceptors})
      : super(channel, options: options,
        interceptors: interceptors);

  $grpc.ResponseStream<$0.Port> connected($async.Stream<$0.DistroInfo> request, {$grpc.CallOptions? options}) {
    return $createStreamingCall(_$connected, request, options: options);
  }
}

@$pb.GrpcServiceName('agentapi.WSLInstance')
abstract class WSLInstanceServiceBase extends $grpc.Service {
  $core.String get $name => 'agentapi.WSLInstance';

  WSLInstanceServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.DistroInfo, $0.Port>(
        'Connected',
        connected,
        true,
        true,
        ($core.List<$core.int> value) => $0.DistroInfo.fromBuffer(value),
        ($0.Port value) => value.writeToBuffer()));
  }

  $async.Stream<$0.Port> connected($grpc.ServiceCall call, $async.Stream<$0.DistroInfo> request);
}
