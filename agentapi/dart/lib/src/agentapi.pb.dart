///
//  Generated code. Do not modify.
//  source: agentapi.proto
//
// @dart = 2.12
// ignore_for_file: annotate_overrides,camel_case_types,constant_identifier_names,directives_ordering,library_prefixes,non_constant_identifier_names,prefer_final_fields,return_of_invalid_type,unnecessary_const,unnecessary_import,unnecessary_this,unused_import,unused_shown_name

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class Empty extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'Empty', package: const $pb.PackageName(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'agentapi'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  Empty._() : super();
  factory Empty() => create();
  factory Empty.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory Empty.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  Empty clone() => Empty()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  Empty copyWith(void Function(Empty) updates) => super.copyWith((message) => updates(message as Empty)) as Empty; // ignore: deprecated_member_use
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static Empty create() => Empty._();
  Empty createEmptyInstance() => create();
  static $pb.PbList<Empty> createRepeated() => $pb.PbList<Empty>();
  @$core.pragma('dart2js:noInline')
  static Empty getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Empty>(create);
  static Empty? _defaultInstance;
}

class ProAttachInfo extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'ProAttachInfo', package: const $pb.PackageName(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'agentapi'), createEmptyInstance: create)
    ..aOS(1, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'token')
    ..hasRequiredFields = false
  ;

  ProAttachInfo._() : super();
  factory ProAttachInfo({
    $core.String? token,
  }) {
    final _result = create();
    if (token != null) {
      _result.token = token;
    }
    return _result;
  }
  factory ProAttachInfo.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ProAttachInfo.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  ProAttachInfo clone() => ProAttachInfo()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  ProAttachInfo copyWith(void Function(ProAttachInfo) updates) => super.copyWith((message) => updates(message as ProAttachInfo)) as ProAttachInfo; // ignore: deprecated_member_use
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ProAttachInfo create() => ProAttachInfo._();
  ProAttachInfo createEmptyInstance() => create();
  static $pb.PbList<ProAttachInfo> createRepeated() => $pb.PbList<ProAttachInfo>();
  @$core.pragma('dart2js:noInline')
  static ProAttachInfo getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProAttachInfo>(create);
  static ProAttachInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => clearField(1);
}

class DistroInfo extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'DistroInfo', package: const $pb.PackageName(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'agentapi'), createEmptyInstance: create)
    ..aOS(1, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'wslName')
    ..aOS(2, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'id')
    ..aOS(3, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'versionId')
    ..aOS(4, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'prettyName')
    ..aOB(5, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'proAttached')
    ..hasRequiredFields = false
  ;

  DistroInfo._() : super();
  factory DistroInfo({
    $core.String? wslName,
    $core.String? id,
    $core.String? versionId,
    $core.String? prettyName,
    $core.bool? proAttached,
  }) {
    final _result = create();
    if (wslName != null) {
      _result.wslName = wslName;
    }
    if (id != null) {
      _result.id = id;
    }
    if (versionId != null) {
      _result.versionId = versionId;
    }
    if (prettyName != null) {
      _result.prettyName = prettyName;
    }
    if (proAttached != null) {
      _result.proAttached = proAttached;
    }
    return _result;
  }
  factory DistroInfo.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory DistroInfo.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  DistroInfo clone() => DistroInfo()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  DistroInfo copyWith(void Function(DistroInfo) updates) => super.copyWith((message) => updates(message as DistroInfo)) as DistroInfo; // ignore: deprecated_member_use
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static DistroInfo create() => DistroInfo._();
  DistroInfo createEmptyInstance() => create();
  static $pb.PbList<DistroInfo> createRepeated() => $pb.PbList<DistroInfo>();
  @$core.pragma('dart2js:noInline')
  static DistroInfo getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DistroInfo>(create);
  static DistroInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get wslName => $_getSZ(0);
  @$pb.TagNumber(1)
  set wslName($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasWslName() => $_has(0);
  @$pb.TagNumber(1)
  void clearWslName() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get id => $_getSZ(1);
  @$pb.TagNumber(2)
  set id($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasId() => $_has(1);
  @$pb.TagNumber(2)
  void clearId() => clearField(2);

  @$pb.TagNumber(3)
  $core.String get versionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set versionId($core.String v) { $_setString(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasVersionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearVersionId() => clearField(3);

  @$pb.TagNumber(4)
  $core.String get prettyName => $_getSZ(3);
  @$pb.TagNumber(4)
  set prettyName($core.String v) { $_setString(3, v); }
  @$pb.TagNumber(4)
  $core.bool hasPrettyName() => $_has(3);
  @$pb.TagNumber(4)
  void clearPrettyName() => clearField(4);

  @$pb.TagNumber(5)
  $core.bool get proAttached => $_getBF(4);
  @$pb.TagNumber(5)
  set proAttached($core.bool v) { $_setBool(4, v); }
  @$pb.TagNumber(5)
  $core.bool hasProAttached() => $_has(4);
  @$pb.TagNumber(5)
  void clearProAttached() => clearField(5);
}

class Port extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'Port', package: const $pb.PackageName(const $core.bool.fromEnvironment('protobuf.omit_message_names') ? '' : 'agentapi'), createEmptyInstance: create)
    ..a<$core.int>(1, const $core.bool.fromEnvironment('protobuf.omit_field_names') ? '' : 'port', $pb.PbFieldType.OU3)
    ..hasRequiredFields = false
  ;

  Port._() : super();
  factory Port({
    $core.int? port,
  }) {
    final _result = create();
    if (port != null) {
      _result.port = port;
    }
    return _result;
  }
  factory Port.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory Port.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  Port clone() => Port()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  Port copyWith(void Function(Port) updates) => super.copyWith((message) => updates(message as Port)) as Port; // ignore: deprecated_member_use
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static Port create() => Port._();
  Port createEmptyInstance() => create();
  static $pb.PbList<Port> createRepeated() => $pb.PbList<Port>();
  @$core.pragma('dart2js:noInline')
  static Port getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Port>(create);
  static Port? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get port => $_getIZ(0);
  @$pb.TagNumber(1)
  set port($core.int v) { $_setUnsignedInt32(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasPort() => $_has(0);
  @$pb.TagNumber(1)
  void clearPort() => clearField(1);
}

