//
//  Generated code. Do not modify.
//  source: agentapi.proto
//
// @dart = 2.12

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
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
  static final _$applyLandscapeConfig = $grpc.ClientMethod<$0.LandscapeConfig, $0.Empty>(
      '/agentapi.UI/ApplyLandscapeConfig',
      ($0.LandscapeConfig value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));
  static final _$ping = $grpc.ClientMethod<$0.Empty, $0.Empty>(
      '/agentapi.UI/Ping',
      ($0.Empty value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));
  static final _$getConfigSources = $grpc.ClientMethod<$0.Empty, $0.ConfigSources>(
      '/agentapi.UI/GetConfigSources',
      ($0.Empty value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.ConfigSources.fromBuffer(value));
  static final _$notifyPurchase = $grpc.ClientMethod<$0.Empty, $0.Empty>(
      '/agentapi.UI/NotifyPurchase',
      ($0.Empty value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));

  UIClient($grpc.ClientChannel channel,
      {$grpc.CallOptions? options,
      $core.Iterable<$grpc.ClientInterceptor>? interceptors})
      : super(channel, options: options,
        interceptors: interceptors);

  $grpc.ResponseFuture<$0.Empty> applyProToken($0.ProAttachInfo request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$applyProToken, request, options: options);
  }

  $grpc.ResponseFuture<$0.Empty> applyLandscapeConfig($0.LandscapeConfig request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$applyLandscapeConfig, request, options: options);
  }

  $grpc.ResponseFuture<$0.Empty> ping($0.Empty request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$ping, request, options: options);
  }

  $grpc.ResponseStream<$0.ConfigSources> getConfigSources($0.Empty request, {$grpc.CallOptions? options}) {
    return $createStreamingCall(_$getConfigSources, $async.Stream.fromIterable([request]), options: options);
  }

  $grpc.ResponseFuture<$0.Empty> notifyPurchase($0.Empty request, {$grpc.CallOptions? options}) {
    return $createUnaryCall(_$notifyPurchase, request, options: options);
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
    $addMethod($grpc.ServiceMethod<$0.LandscapeConfig, $0.Empty>(
        'ApplyLandscapeConfig',
        applyLandscapeConfig_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LandscapeConfig.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Empty, $0.Empty>(
        'Ping',
        ping_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Empty.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Empty, $0.ConfigSources>(
        'GetConfigSources',
        getConfigSources_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.Empty.fromBuffer(value),
        ($0.ConfigSources value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Empty, $0.Empty>(
        'NotifyPurchase',
        notifyPurchase_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Empty.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
  }

  $async.Future<$0.Empty> applyProToken_Pre($grpc.ServiceCall call, $async.Future<$0.ProAttachInfo> request) async {
    return applyProToken(call, await request);
  }

  $async.Future<$0.Empty> applyLandscapeConfig_Pre($grpc.ServiceCall call, $async.Future<$0.LandscapeConfig> request) async {
    return applyLandscapeConfig(call, await request);
  }

  $async.Future<$0.Empty> ping_Pre($grpc.ServiceCall call, $async.Future<$0.Empty> request) async {
    return ping(call, await request);
  }

  $async.Stream<$0.ConfigSources> getConfigSources_Pre($grpc.ServiceCall call, $async.Future<$0.Empty> request) async* {
    yield* getConfigSources(call, await request);
  }

  $async.Future<$0.Empty> notifyPurchase_Pre($grpc.ServiceCall call, $async.Future<$0.Empty> request) async {
    return notifyPurchase(call, await request);
  }

  $async.Future<$0.Empty> applyProToken($grpc.ServiceCall call, $0.ProAttachInfo request);
  $async.Future<$0.Empty> applyLandscapeConfig($grpc.ServiceCall call, $0.LandscapeConfig request);
  $async.Future<$0.Empty> ping($grpc.ServiceCall call, $0.Empty request);
  $async.Stream<$0.ConfigSources> getConfigSources($grpc.ServiceCall call, $0.Empty request);
  $async.Future<$0.Empty> notifyPurchase($grpc.ServiceCall call, $0.Empty request);
}
@$pb.GrpcServiceName('agentapi.WSLInstance')
class WSLInstanceClient extends $grpc.Client {
  static final _$connected = $grpc.ClientMethod<$0.DistroInfo, $0.Empty>(
      '/agentapi.WSLInstance/Connected',
      ($0.DistroInfo value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.Empty.fromBuffer(value));
  static final _$proAttachmentCommands = $grpc.ClientMethod<$0.MSG, $0.ProAttachCmd>(
      '/agentapi.WSLInstance/ProAttachmentCommands',
      ($0.MSG value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.ProAttachCmd.fromBuffer(value));
  static final _$landscapeConfigCommands = $grpc.ClientMethod<$0.MSG, $0.LandscapeConfigCmd>(
      '/agentapi.WSLInstance/LandscapeConfigCommands',
      ($0.MSG value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.LandscapeConfigCmd.fromBuffer(value));

  WSLInstanceClient($grpc.ClientChannel channel,
      {$grpc.CallOptions? options,
      $core.Iterable<$grpc.ClientInterceptor>? interceptors})
      : super(channel, options: options,
        interceptors: interceptors);

  $grpc.ResponseFuture<$0.Empty> connected($async.Stream<$0.DistroInfo> request, {$grpc.CallOptions? options}) {
    return $createStreamingCall(_$connected, request, options: options).single;
  }

  $grpc.ResponseStream<$0.ProAttachCmd> proAttachmentCommands($async.Stream<$0.MSG> request, {$grpc.CallOptions? options}) {
    return $createStreamingCall(_$proAttachmentCommands, request, options: options);
  }

  $grpc.ResponseStream<$0.LandscapeConfigCmd> landscapeConfigCommands($async.Stream<$0.MSG> request, {$grpc.CallOptions? options}) {
    return $createStreamingCall(_$landscapeConfigCommands, request, options: options);
  }
}

@$pb.GrpcServiceName('agentapi.WSLInstance')
abstract class WSLInstanceServiceBase extends $grpc.Service {
  $core.String get $name => 'agentapi.WSLInstance';

  WSLInstanceServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.DistroInfo, $0.Empty>(
        'Connected',
        connected,
        true,
        false,
        ($core.List<$core.int> value) => $0.DistroInfo.fromBuffer(value),
        ($0.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MSG, $0.ProAttachCmd>(
        'ProAttachmentCommands',
        proAttachmentCommands,
        true,
        true,
        ($core.List<$core.int> value) => $0.MSG.fromBuffer(value),
        ($0.ProAttachCmd value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MSG, $0.LandscapeConfigCmd>(
        'LandscapeConfigCommands',
        landscapeConfigCommands,
        true,
        true,
        ($core.List<$core.int> value) => $0.MSG.fromBuffer(value),
        ($0.LandscapeConfigCmd value) => value.writeToBuffer()));
  }

  $async.Future<$0.Empty> connected($grpc.ServiceCall call, $async.Stream<$0.DistroInfo> request);
  $async.Stream<$0.ProAttachCmd> proAttachmentCommands($grpc.ServiceCall call, $async.Stream<$0.MSG> request);
  $async.Stream<$0.LandscapeConfigCmd> landscapeConfigCommands($grpc.ServiceCall call, $async.Stream<$0.MSG> request);
}
