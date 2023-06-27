//
//  Generated code. Do not modify.
//  source: agentapi.proto
//
// @dart = 2.12

// ignore_for_file: annotate_overrides, camel_case_types
// ignore_for_file: constant_identifier_names, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_final_fields
// ignore_for_file: unnecessary_import, unnecessary_this, unused_import

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class Empty extends $pb.GeneratedMessage {
  factory Empty() => create();
  Empty._() : super();
  factory Empty.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory Empty.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(_omitMessageNames ? '' : 'Empty', package: const $pb.PackageName(_omitMessageNames ? '' : 'agentapi'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  Empty clone() => Empty()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  Empty copyWith(void Function(Empty) updates) => super.copyWith((message) => updates(message as Empty)) as Empty;

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
  factory ProAttachInfo() => create();
  ProAttachInfo._() : super();
  factory ProAttachInfo.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ProAttachInfo.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(_omitMessageNames ? '' : 'ProAttachInfo', package: const $pb.PackageName(_omitMessageNames ? '' : 'agentapi'), createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..hasRequiredFields = false
  ;

  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  ProAttachInfo clone() => ProAttachInfo()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  ProAttachInfo copyWith(void Function(ProAttachInfo) updates) => super.copyWith((message) => updates(message as ProAttachInfo)) as ProAttachInfo;

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

enum SubscriptionInfo_SubscriptionType {
  none, 
  manual, 
  microsoftStore, 
  notSet
}

class SubscriptionInfo extends $pb.GeneratedMessage {
  factory SubscriptionInfo() => create();
  SubscriptionInfo._() : super();
  factory SubscriptionInfo.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory SubscriptionInfo.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);

  static const $core.Map<$core.int, SubscriptionInfo_SubscriptionType> _SubscriptionInfo_SubscriptionTypeByTag = {
    3 : SubscriptionInfo_SubscriptionType.none,
    4 : SubscriptionInfo_SubscriptionType.manual,
    5 : SubscriptionInfo_SubscriptionType.microsoftStore,
    0 : SubscriptionInfo_SubscriptionType.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(_omitMessageNames ? '' : 'SubscriptionInfo', package: const $pb.PackageName(_omitMessageNames ? '' : 'agentapi'), createEmptyInstance: create)
    ..oo(0, [3, 4, 5])
    ..aOS(1, _omitFieldNames ? '' : 'productId', protoName: 'productId')
    ..aOB(2, _omitFieldNames ? '' : 'userManaged', protoName: 'userManaged')
    ..aOM<Empty>(3, _omitFieldNames ? '' : 'none', subBuilder: Empty.create)
    ..aOM<Empty>(4, _omitFieldNames ? '' : 'manual', subBuilder: Empty.create)
    ..aOM<Empty>(5, _omitFieldNames ? '' : 'microsoftStore', protoName: 'microsoftStore', subBuilder: Empty.create)
    ..hasRequiredFields = false
  ;

  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  SubscriptionInfo clone() => SubscriptionInfo()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  SubscriptionInfo copyWith(void Function(SubscriptionInfo) updates) => super.copyWith((message) => updates(message as SubscriptionInfo)) as SubscriptionInfo;

  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubscriptionInfo create() => SubscriptionInfo._();
  SubscriptionInfo createEmptyInstance() => create();
  static $pb.PbList<SubscriptionInfo> createRepeated() => $pb.PbList<SubscriptionInfo>();
  @$core.pragma('dart2js:noInline')
  static SubscriptionInfo getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SubscriptionInfo>(create);
  static SubscriptionInfo? _defaultInstance;

  SubscriptionInfo_SubscriptionType whichSubscriptionType() => _SubscriptionInfo_SubscriptionTypeByTag[$_whichOneof(0)]!;
  void clearSubscriptionType() => clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => clearField(1);

  @$pb.TagNumber(2)
  $core.bool get userManaged => $_getBF(1);
  @$pb.TagNumber(2)
  set userManaged($core.bool v) { $_setBool(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasUserManaged() => $_has(1);
  @$pb.TagNumber(2)
  void clearUserManaged() => clearField(2);

  @$pb.TagNumber(3)
  Empty get none => $_getN(2);
  @$pb.TagNumber(3)
  set none(Empty v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasNone() => $_has(2);
  @$pb.TagNumber(3)
  void clearNone() => clearField(3);
  @$pb.TagNumber(3)
  Empty ensureNone() => $_ensure(2);

  @$pb.TagNumber(4)
  Empty get manual => $_getN(3);
  @$pb.TagNumber(4)
  set manual(Empty v) { setField(4, v); }
  @$pb.TagNumber(4)
  $core.bool hasManual() => $_has(3);
  @$pb.TagNumber(4)
  void clearManual() => clearField(4);
  @$pb.TagNumber(4)
  Empty ensureManual() => $_ensure(3);

  @$pb.TagNumber(5)
  Empty get microsoftStore => $_getN(4);
  @$pb.TagNumber(5)
  set microsoftStore(Empty v) { setField(5, v); }
  @$pb.TagNumber(5)
  $core.bool hasMicrosoftStore() => $_has(4);
  @$pb.TagNumber(5)
  void clearMicrosoftStore() => clearField(5);
  @$pb.TagNumber(5)
  Empty ensureMicrosoftStore() => $_ensure(4);
}

class DistroInfo extends $pb.GeneratedMessage {
  factory DistroInfo() => create();
  DistroInfo._() : super();
  factory DistroInfo.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory DistroInfo.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(_omitMessageNames ? '' : 'DistroInfo', package: const $pb.PackageName(_omitMessageNames ? '' : 'agentapi'), createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'wslName')
    ..aOS(2, _omitFieldNames ? '' : 'id')
    ..aOS(3, _omitFieldNames ? '' : 'versionId')
    ..aOS(4, _omitFieldNames ? '' : 'prettyName')
    ..aOB(5, _omitFieldNames ? '' : 'proAttached')
    ..aOS(6, _omitFieldNames ? '' : 'hostname')
    ..hasRequiredFields = false
  ;

  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  DistroInfo clone() => DistroInfo()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  DistroInfo copyWith(void Function(DistroInfo) updates) => super.copyWith((message) => updates(message as DistroInfo)) as DistroInfo;

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

  @$pb.TagNumber(6)
  $core.String get hostname => $_getSZ(5);
  @$pb.TagNumber(6)
  set hostname($core.String v) { $_setString(5, v); }
  @$pb.TagNumber(6)
  $core.bool hasHostname() => $_has(5);
  @$pb.TagNumber(6)
  void clearHostname() => clearField(6);
}

class Port extends $pb.GeneratedMessage {
  factory Port() => create();
  Port._() : super();
  factory Port.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory Port.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(_omitMessageNames ? '' : 'Port', package: const $pb.PackageName(_omitMessageNames ? '' : 'agentapi'), createEmptyInstance: create)
    ..a<$core.int>(1, _omitFieldNames ? '' : 'port', $pb.PbFieldType.OU3)
    ..hasRequiredFields = false
  ;

  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.deepCopy] instead. '
  'Will be removed in next major version')
  Port clone() => Port()..mergeFromMessage(this);
  @$core.Deprecated(
  'Using this can add significant overhead to your binary. '
  'Use [GeneratedMessageGenericExtensions.rebuild] instead. '
  'Will be removed in next major version')
  Port copyWith(void Function(Port) updates) => super.copyWith((message) => updates(message as Port)) as Port;

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


const _omitFieldNames = $core.bool.fromEnvironment('protobuf.omit_field_names');
const _omitMessageNames = $core.bool.fromEnvironment('protobuf.omit_message_names');
